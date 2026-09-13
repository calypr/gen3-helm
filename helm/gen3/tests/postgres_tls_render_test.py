#!/usr/bin/env python3
"""Render-level regression tests for the umbrella PostgreSQL TLS contract."""

import base64
import pathlib
import ssl
import subprocess
import tempfile
import time
import unittest

import yaml


CHART = pathlib.Path(__file__).resolve().parents[1]

# The external-PostgreSQL cases do not need unrelated application dependencies.
# Disabling them keeps these tests focused and avoids requiring their credentials.
EXTERNAL_DISABLED = (
    "ambassador",
    "arborist",
    "argo-wrapper",
    "aws-es-proxy",
    "frontend-framework",
    "fence",
    "fhir-server",
    "funnel",
    "gecko",
    "grip",
    "guppy",
    "hatchery",
    "kafka",
    "requestor",
    "redis",
    "revproxy",
    "sower",
    "elasticsearch",
    "image-viewer",
    "viv",
    "loom",
)


def render(*overrides, api_versions=(), external=False):
    command = [
        "helm",
        "template",
        "tls-test",
        str(CHART),
        "--namespace",
        "tls-ns",
        "--set",
        "syfon.config.credential_encryption.master_key=test-key",
    ]
    if external:
        command.extend(["--set", "global.dev=false"])
        for dependency in EXTERNAL_DISABLED:
            command.extend(["--set", f"{dependency}.enabled=false"])
    for api_version in api_versions:
        command.extend(["--api-versions", api_version])
    for override in overrides:
        command.extend(["--set", override])
    return subprocess.run(command, check=True, text=True, capture_output=True).stdout


def resources(manifest):
    return [resource for resource in yaml.safe_load_all(manifest) if resource]


def resource(resources_list, kind, name):
    return next(
        item
        for item in resources_list
        if item.get("kind") == kind and item.get("metadata", {}).get("name") == name
    )


def env_values(pod):
    return {
        item["name"]: item["value"]
        for item in pod["spec"]["containers"][0].get("env", [])
        if "value" in item
    }


class PostgresTLSRenderTests(unittest.TestCase):
    def test_self_signed_renders_san_certificate_and_shared_trust(self):
        rendered = resources(render("global.postgres.tls.mode=selfSigned"))
        tls_secret = resource(rendered, "Secret", "gen3-postgresql-tls")
        leaf = base64.b64decode(tls_secret["data"]["tls.crt"])
        self.assertIn(b"BEGIN CERTIFICATE", leaf)
        self.assertIn(b"BEGIN CERTIFICATE", base64.b64decode(tls_secret["data"]["ca.crt"]))
        with tempfile.NamedTemporaryFile() as certificate_file:
            certificate_file.write(leaf)
            certificate_file.flush()
            decoded = ssl._ssl._test_decode_cert(certificate_file.name)
        self.assertGreater(
            ssl.cert_time_to_seconds(decoded["notAfter"]),
            time.time() + (9 * 365 * 24 * 60 * 60),
        )
        sans = sorted(value for kind, value in decoded["subjectAltName"] if kind == "DNS")
        self.assertEqual(
            sans,
            sorted(
                [
                    "tls-test-postgresql",
                    "tls-test-postgresql.tls-ns",
                    "tls-test-postgresql.tls-ns.svc",
                    "tls-test-postgresql.tls-ns.svc.cluster.local",
                    "tls-test-postgresql-hl",
                    "tls-test-postgresql-hl.tls-ns",
                    "tls-test-postgresql-hl.tls-ns.svc",
                    "tls-test-postgresql-hl.tls-ns.svc.cluster.local",
                ]
            ),
        )

        deployment = resource(rendered, "Deployment", "syfon-deployment")
        job = resource(rendered, "Job", "syfon-postgres-init-1")
        for pod in (deployment["spec"]["template"], job["spec"]["template"]):
            env = env_values(pod)
            self.assertEqual(env["PGSSLMODE"], "verify-full")
            self.assertEqual(env["PGSSLROOTCERT"], "/etc/ssl/certs/postgres/ca.crt")
            self.assertIn(
                {
                    "name": "postgres-ca",
                    "mountPath": "/etc/ssl/certs/postgres",
                    "readOnly": True,
                },
                pod["spec"]["containers"][0]["volumeMounts"],
            )
        postgres = resource(rendered, "StatefulSet", "tls-test-postgresql")
        postgres_env = env_values(postgres["spec"]["template"])
        self.assertEqual(postgres_env["POSTGRESQL_ENABLE_TLS"], "yes")
        self.assertNotIn("POSTGRESQL_TLS_CA_FILE", postgres_env)

    def test_cert_manager_certificate_contains_service_sans(self):
        rendered = resources(
            render(
                "global.postgres.tls.mode=certManager",
                "global.postgres.tls.certManager.issuerRef.name=internal-ca",
                "postgresql.clusterDomain=cluster.internal",
                api_versions=("cert-manager.io/v1",),
            )
        )
        certificate = resource(rendered, "Certificate", "gen3-postgresql-tls-certificate")
        self.assertEqual(
            certificate["spec"]["dnsNames"],
            [
                "tls-test-postgresql",
                "tls-test-postgresql.tls-ns",
                "tls-test-postgresql.tls-ns.svc",
                "tls-test-postgresql.tls-ns.svc.cluster.internal",
                "tls-test-postgresql-hl",
                "tls-test-postgresql-hl.tls-ns",
                "tls-test-postgresql-hl.tls-ns.svc",
                "tls-test-postgresql-hl.tls-ns.svc.cluster.internal",
            ],
        )

    def test_existing_secret_external_mode_mounts_caller_ca(self):
        rendered = resources(
            render(
                "global.postgres.tls.mode=existingSecret",
                "global.postgres.tls.secretName=caller-postgres-tls",
                "global.postgres.tls.caKey=postgres-ca.pem",
                external=True,
            )
        )
        self.assertFalse(any(item.get("kind") == "StatefulSet" for item in rendered))
        self.assertFalse(any(item.get("kind") == "Certificate" for item in rendered))
        deployment = resource(rendered, "Deployment", "syfon-deployment")
        job = resource(rendered, "Job", "syfon-postgres-init-1")
        for pod in (deployment["spec"]["template"], job["spec"]["template"]):
            volume = next(item for item in pod["spec"]["volumes"] if item["name"] == "postgres-ca")
            self.assertEqual(volume["secret"]["secretName"], "caller-postgres-tls")
            self.assertEqual(volume["secret"]["items"][0]["key"], "postgres-ca.pem")

    def test_existing_secret_bundled_mode_uses_the_canonical_server_secret(self):
        rendered = resources(render("global.postgres.tls.mode=existingSecret"))
        self.assertFalse(any(item.get("kind") == "Certificate" for item in rendered))
        self.assertFalse(
            any(
                item.get("kind") == "Secret"
                and item.get("metadata", {}).get("name") == "gen3-postgresql-tls"
                for item in rendered
            )
        )
        postgres = resource(rendered, "StatefulSet", "tls-test-postgresql")
        raw_certificate_volume = next(
            volume
            for volume in postgres["spec"]["template"]["spec"]["volumes"]
            if volume["name"] == "raw-certificates"
        )
        self.assertEqual(raw_certificate_volume["secret"]["secretName"], "gen3-postgresql-tls")

    def test_external_mode_without_syfon_skips_syfon_tls_validation(self):
        rendered = resources(
            render(
                "global.postgres.tls.mode=selfSigned",
                "global.postgres.tls.secretName=unused-postgres-tls",
                "syfon.enabled=false",
                external=True,
            )
        )
        self.assertFalse(any(item.get("kind") == "Certificate" for item in rendered))
        self.assertFalse(any(item.get("kind") == "StatefulSet" for item in rendered))
        self.assertFalse(any(item.get("kind") == "Deployment" for item in rendered))

    def test_invalid_tls_combinations_fail_render(self):
        invalid = (
            (("global.postgres.tls.mode=disabled",), ()),
            (("global.dev=false", "global.postgres.tls.mode=selfSigned", "global.postgres.tls.secretName=external"), ()),
            (
                (
                    "global.dev=false",
                    "global.postgres.tls.mode=certManager",
                    "global.postgres.tls.secretName=external",
                    "global.postgres.tls.certManager.issuerRef.name=internal-ca",
                ),
                (),
            ),
            (("global.postgres.tls.mode=existingSecret", "global.postgres.tls.secretName=another-server-secret"), ()),
            (
                ("global.postgres.tls.mode=certManager", "global.postgres.tls.certManager.issuerRef.name=internal-ca"),
                (),
            ),
            (("postgresql.tls.autoGenerated=true",), ()),
            (("postgresql.tls.certCAFilename=ca.crt",), ()),
        )
        for overrides, api_versions in invalid:
            with self.subTest(overrides=overrides):
                with self.assertRaises(subprocess.CalledProcessError):
                    render(
                        *overrides,
                        external=any(value == "global.dev=false" for value in overrides),
                        api_versions=api_versions,
                    )


if __name__ == "__main__":
    unittest.main()
