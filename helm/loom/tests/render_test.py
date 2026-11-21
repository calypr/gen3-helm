#!/usr/bin/env python3
"""Render-level regression tests for the Loom recipe and image contract."""

import json
import pathlib
import subprocess
import unittest

import yaml


CHART = pathlib.Path(__file__).resolve().parents[1]


def render(*overrides):
    return subprocess.run(
        ["helm", "template", "loom", str(CHART), "--namespace", "loom-test",
         "--set", "global.hostname=loom.test", *overrides],
        check=True,
        text=True,
        capture_output=True,
    ).stdout


def resources(manifest):
    return [resource for resource in yaml.safe_load_all(manifest) if resource]


def resource(rendered, kind, name):
    return next(
        item for item in rendered
        if item["kind"] == kind and item["metadata"]["name"] == name
    )


def volume(workload, name):
    return next(
        item for item in workload["spec"]["template"]["spec"]["volumes"]
        if item["name"] == name
    )


class LoomRenderTests(unittest.TestCase):
    def test_default_extension_recipe_uses_chart_image(self):
        rendered = resources(render())
        secret = next(
            resource for resource in rendered
            if resource["kind"] == "Secret" and resource["metadata"]["name"] == "loom-config"
        )
        recipe = json.loads(secret["stringData"]["dataframer.json"])
        self.assertIn("extensionColumns", recipe["outputs"][0])
        self.assertNotIn("project-configs.json", secret["stringData"])
        config = yaml.safe_load(secret["stringData"]["config.yaml"])
        self.assertNotIn("project_configs", config["server"])

        deployment = resource(rendered, "Deployment", "loom-deployment")
        container = next(
            item for item in deployment["spec"]["template"]["spec"]["containers"]
            if item["name"] == "loom"
        )
        app_version = yaml.safe_load((CHART / "Chart.yaml").read_text())["appVersion"]
        self.assertEqual(container["image"], f"quay.io/ohsu-comp-bio/loom:sha-{app_version}")
        self.assertEqual(container["imagePullPolicy"], "IfNotPresent")


    def test_database_storage_defaults_to_empty_dir(self):
        rendered = resources(render())
        arango = resource(rendered, "Deployment", "arangodb")
        clickhouse = resource(rendered, "StatefulSet", "clickhouse")
        self.assertIn("emptyDir", volume(arango, "data"))
        self.assertIn("emptyDir", volume(clickhouse, "clickhouse-data"))

    def test_chart_provisions_database_claims(self):
        rendered = resources(render(
            "--set", "arango.persistence.enabled=true",
            "--set-string", "arango.persistence.storageClass=",
            "--set", "clickstack.persistence.enabled=true",
        ))
        arango = resource(rendered, "Deployment", "arangodb")
        claim = resource(rendered, "PersistentVolumeClaim", "loom-arango-data")
        self.assertEqual(arango["spec"]["strategy"]["type"], "Recreate")
        self.assertEqual(volume(arango, "data")["persistentVolumeClaim"]["claimName"],
                         claim["metadata"]["name"])
        self.assertEqual(claim["spec"]["storageClassName"], "")
        self.assertEqual(claim["spec"]["resources"]["requests"]["storage"], "30Gi")
        clickhouse = resource(rendered, "StatefulSet", "clickhouse")
        claim_template = clickhouse["spec"]["volumeClaimTemplates"][0]
        self.assertEqual(claim_template["metadata"]["name"], "clickhouse-data")
        self.assertEqual(claim_template["spec"]["resources"]["requests"]["storage"], "6Gi")

    def test_database_claims_use_the_selected_storage_class(self):
        for storage_class in (None, "database-disks", ""):
            with self.subTest(storage_class=storage_class):
                overrides = [
                    "--set", "arango.persistence.enabled=true",
                    "--set", "clickstack.persistence.enabled=true",
                ]
                if storage_class is not None:
                    overrides.extend([
                        "--set-string", f"arango.persistence.storageClass={storage_class}",
                        "--set-string", f"clickstack.persistence.storageClass={storage_class}",
                    ])
                rendered = resources(render(*overrides))
                arango = resource(rendered, "PersistentVolumeClaim", "loom-arango-data")["spec"]
                clickhouse = resource(rendered, "StatefulSet", "clickhouse")["spec"]
                clickhouse_claim = clickhouse["volumeClaimTemplates"][0]["spec"]
                for claim in (arango, clickhouse_claim):
                    if storage_class is None:
                        self.assertNotIn("storageClassName", claim)
                    else:
                        self.assertEqual(claim["storageClassName"], storage_class)
                self.assertFalse(any(item["kind"] == "PersistentVolume" for item in rendered))

    def test_databases_can_mount_existing_claims(self):
        rendered = resources(render(
            "--set", "arango.persistence.enabled=true",
            "--set", "arango.persistence.existingClaim=dev-arango",
            "--set", "clickstack.persistence.enabled=true",
            "--set", "clickstack.persistence.existingClaim=dev-clickhouse",
        ))
        arango = resource(rendered, "Deployment", "arangodb")
        clickhouse = resource(rendered, "StatefulSet", "clickhouse")
        self.assertEqual(volume(arango, "data")["persistentVolumeClaim"]["claimName"],
                         "dev-arango")
        self.assertEqual(volume(clickhouse, "clickhouse-data")["persistentVolumeClaim"]["claimName"],
                         "dev-clickhouse")
        self.assertFalse(any(item["kind"] == "PersistentVolumeClaim" for item in rendered))
        self.assertNotIn("volumeClaimTemplates", clickhouse["spec"])


if __name__ == "__main__":
    unittest.main()
