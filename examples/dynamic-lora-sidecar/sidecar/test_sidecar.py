import unittest
from unittest.mock import patch, Mock, mock_open
import yaml
from sidecar import LoraReconciler, CONFIG_MAP_FILE

TEST_CONFIG_DATA = {
    "deployment": {
        "name": "test-deployment",
        "host": "localhost",
        "port": "8000",
        "models": [
            {"id": "lora1", "source": "/path/to/lora1", "base-model": "base1"},
            {
                "id": "lora2",
                "source": "/path/to/lora2",
                "base-model": "base1",
                "toRemove": True,
            },
        ],
    }
}

RESPONSES = {
    "v1/models": {
        "object": "list",
        "data": [
            {
                "id": "base1",
                "object": "model",
                "created": 1729693000,
                "owned_by": "vllm",
                "root": "meta-llama/Llama-2-7b-hf",
                "parent": None,
                "max_model_len": 4096,
            },
            {
                "id": "lora2",
                "object": "model",
                "created": 1729693000,
                "owned_by": "vllm",
                "root": "yard1/llama-2-7b-sql-lora-test",
                "parent": "base1",
                "max_model_len": None,
            },
        ],
    },
}
REGISTERED_ADAPTERS = {
    "base1": {
        "created": 1729693000,
        "id": "base1",
        "max_model_len": 4096,
        "object": "model",
        "owned_by": "vllm",
        "parent": None,
        "root": "meta-llama/Llama-2-7b-hf",
    },
    "lora2": {
        "created": 1729693000,
        "id": "lora2",
        "max_model_len": None,
        "object": "model",
        "owned_by": "vllm",
        "parent": "base1",
        "root": "yard1/llama-2-7b-sql-lora-test",
    },
}


def getMockResponse(status_return_value: object = None) -> object:
    mock_response = Mock()
    mock_response.raise_for_status.return_value = None
    return mock_response


class LoraReconcilerTest(unittest.TestCase):
    @patch(
        "builtins.open", new_callable=mock_open, read_data=yaml.dump(TEST_CONFIG_DATA)
    )
    @patch("sidecar.requests.get")
    def setUp(self, mock_get, mock_file):
        mock_response = getMockResponse()
        mock_response.json.return_value = RESPONSES["v1/models"]
        mock_get.return_value = mock_response
        self.reconciler = LoraReconciler()
        self.maxDiff = None
        mock_file.assert_called_once_with(CONFIG_MAP_FILE, "r")

    @patch("sidecar.requests.get")
    def test_get_registered_adapters(self, mock_get):
        mock_response = getMockResponse()
        mock_response.json.return_value = RESPONSES["v1/models"]
        mock_get.return_value = mock_response

        self.reconciler.get_registered_adapters()
        self.assertEqual(REGISTERED_ADAPTERS, self.reconciler.registered_adapters)

    @patch("sidecar.requests.post")
    def test_load_adapter(self, mock_post):
        mock_post.return_value = getMockResponse()

        # loading a new adapter
        result = self.reconciler.load_adapter(
            TEST_CONFIG_DATA["deployment"]["models"][0]
        )
        self.assertEqual(result, "")

        # loading an already loaded adapter
        self.reconciler.registered_adapters["lora1"] = {"id": "lora1"}
        result = self.reconciler.load_adapter(
            TEST_CONFIG_DATA["deployment"]["models"][0]
        )
        self.assertEqual(result, "already loaded")

    @patch("sidecar.requests.post")
    def test_unload_adapter(self, mock_post):
        mock_post.return_value = getMockResponse()

        # unloading an existing adapter
        self.reconciler.registered_adapters["lora2"] = {"id": "lora2"}
        result = self.reconciler.unload_adapter(
            TEST_CONFIG_DATA["deployment"]["models"][1]
        )
        self.assertEqual(result, None)

        # unloading an already unloaded adapter
        result = self.reconciler.unload_adapter(
            TEST_CONFIG_DATA["deployment"]["models"][1]
        )
        self.assertEqual(result, "already unloaded")
    
    @patch("builtins.open", new_callable=mock_open, read_data=yaml.dump(TEST_CONFIG_DATA))
    @patch("sidecar.requests.get")
    @patch("sidecar.requests.post")
    def test_reconcile(self, mock_post, mock_get, mock_file):
        mock_get_response = getMockResponse()
        mock_get_response.json.return_value = RESPONSES["v1/models"]
        mock_get.return_value = mock_get_response
        mock_post.return_value = getMockResponse()

        self.reconciler = LoraReconciler()
        self.reconciler.reconcile()

        mock_post.assert_any_call(
            "http://localhost:8000/v1/load_lora_adapter",
            json={
                "lora_name": "lora1",
                "lora_path": "/path/to/lora1",
                "base_model_name": "base1",
            },
        )
        mock_post.assert_any_call(
            "http://localhost:8000/v1/unload_lora_adapter", json={"lora_name": "lora2"}
        )
        updated_config = self.reconciler.config_map_adapters
        mock_file.return_value.write.side_effect = lambda data: data
        self.reconciler.update_status_config()
        mock_file.return_value.write.assert_called()
        self.assertTrue("timestamp" in updated_config["lora1"]["status"])
        self.assertTrue("status" in updated_config["lora2"])


if __name__ == "__main__":
    unittest.main()
