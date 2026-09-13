#!/usr/bin/env python3
"""Render-level regression tests for the Syfon PostgreSQL trust contract."""

import pathlib
import subprocess
import unittest

import yaml


CHART = pathlib.Path(__file__).resolve().parents[1]


def render(*overrides):
    command = [
        "helm",
        "template",
        "syfon",
        str(CHART),
        "--namespace",
        "test-ns",
        "--set",
        "config.credential_encryption.master_key=test-key",
    ]
    for override in overrides:
        command.extend(["--set", override])
    return subprocess.run(command, check=True, text=True, capture_output=True).stdout


def resources(manifest):
    return [resource for resource in yaml.safe_load_all(manifest) if resource]


class PostgresTLSRenderTests(unittest.TestCase):
    def test_self_signed_mode_configures_deployment_and_init_job_trust(self):
        rendered = resources(
            render(
                "global.postgres.tls.mode=selfSigned",
                "global.postgres.tls.secretName=postgres-tls",
            )
        )
        deployment = next(resource for resource in rendered if resource["kind"] == "Deployment")
        job = next(resource for resource in rendered if resource["kind"] == "Job")

        for pod in (deployment["spec"]["template"], job["spec"]["template"]):
            container = pod["spec"]["containers"][0]
            env = {item["name"]: item["value"] for item in container["env"] if "value" in item}
            self.assertEqual(env["PGSSLMODE"], "verify-full")
            self.assertEqual(env["PGSSLROOTCERT"], "/etc/ssl/certs/postgres/ca.crt")
            self.assertIn(
                {"name": "postgres-ca", "mountPath": "/etc/ssl/certs/postgres", "readOnly": True},
                container["volumeMounts"],
            )

    def test_existing_secret_mode_requires_a_secret_name(self):
        with self.assertRaises(subprocess.CalledProcessError):
            render("global.postgres.tls.mode=existingSecret")

    def test_existing_secret_mode_mounts_the_caller_ca(self):
        rendered = resources(
            render(
                "global.postgres.tls.mode=existingSecret",
                "global.postgres.tls.secretName=caller-postgres-tls",
                "global.postgres.tls.caKey=postgres-ca.pem",
            )
        )
        deployment = next(resource for resource in rendered if resource["kind"] == "Deployment")
        job = next(resource for resource in rendered if resource["kind"] == "Job")
        for pod in (deployment["spec"]["template"], job["spec"]["template"]):
            volume = next(item for item in pod["spec"]["volumes"] if item["name"] == "postgres-ca")
            self.assertEqual(volume["secret"]["secretName"], "caller-postgres-tls")
            self.assertEqual(volume["secret"]["items"][0]["key"], "postgres-ca.pem")

    def test_disabled_mode_omits_root_cert_and_ca_mount(self):
        rendered = resources(render("global.postgres.tls.mode=disabled"))
        deployment = next(resource for resource in rendered if resource["kind"] == "Deployment")
        job = next(resource for resource in rendered if resource["kind"] == "Job")
        for pod in (deployment["spec"]["template"], job["spec"]["template"]):
            env = {item["name"] for item in pod["spec"]["containers"][0]["env"]}
            self.assertNotIn("PGSSLROOTCERT", env)
            self.assertNotIn("postgres-ca", {item["name"] for item in pod["spec"]["volumes"]})

    def test_deprecated_tls_values_cannot_contradict_tls_mode(self):
        with self.assertRaises(subprocess.CalledProcessError):
            render(
                "global.postgres.tls.mode=selfSigned",
                "global.postgres.tls.secretName=postgres-tls",
                "postgres.app.db_sslmode=disable",
            )


if __name__ == "__main__":
    unittest.main()
