#!/usr/bin/env python3
"""Render-level regression tests for the Loom recipe and image contract."""

import json
import pathlib
import subprocess
import unittest

import yaml


CHART = pathlib.Path(__file__).resolve().parents[1]


def render():
    return subprocess.run(
        ["helm", "template", "loom", str(CHART), "--namespace", "loom-test"],
        check=True,
        text=True,
        capture_output=True,
    ).stdout


def resources(manifest):
    return [resource for resource in yaml.safe_load_all(manifest) if resource]


class LoomRenderTests(unittest.TestCase):
    def test_default_extension_recipe_uses_compatible_image(self):
        rendered = resources(render())
        secret = next(
            resource for resource in rendered
            if resource["kind"] == "Secret" and resource["metadata"]["name"] == "loom-config"
        )
        recipe = json.loads(secret["stringData"]["dataframer.json"])
        self.assertIn("extensionColumns", recipe["outputs"][0])

        deployment = next(resource for resource in rendered if resource["kind"] == "Deployment")
        container = next(
            item for item in deployment["spec"]["template"]["spec"]["containers"]
            if item["name"] == "loom"
        )
        self.assertEqual(container["image"], "quay.io/ohsu-comp-bio/loom:sha-f6d8ece")
        self.assertEqual(container["imagePullPolicy"], "IfNotPresent")


if __name__ == "__main__":
    unittest.main()
