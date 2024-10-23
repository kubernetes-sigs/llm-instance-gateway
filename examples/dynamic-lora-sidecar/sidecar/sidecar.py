import requests
import yaml
import time
from watchdog.observers import Observer
from watchdog.events import FileSystemEventHandler
import logging
import datetime
import os

CONFIG_MAP_FILE = os.environ.get('DYNAMIC_LORA_ROLLOUT_CONFIG',"configmap.yaml")
logging.basicConfig(level=logging.INFO, 
                    format='%(asctime)s - %(levelname)s - %(message)s')

def current_time_human() -> str:
    now = datetime.datetime.now(datetime.timezone.utc).astimezone()
    return now.strftime("%Y-%m-%d %H:%M:%S %Z%z")


class ConfigWatcher(FileSystemEventHandler):
    def __init__(self, callback):
        """
        Watches config

        Args:
            callback : Callback function taking no parameters and no return values parsed
        """
        self.callback = callback

    def on_modified(self, event):
        if not event.is_directory and event.src_path.endswith(".yaml"):
            logging.info(
                f"Config '{event.src_path}' modified! at '{current_time_human()}'"
            )
            self.callback()


class LoraReconciler:
    """
    Reconciles adapters registered on vllm server with adapters listed in configmap in current state
    """

    def __init__(self):
        self.deployment_name = ""
        self.registered_adapters = {}
        self.config_map_adapters = {}
        self.load_configmap()
        self.get_registered_adapters()

    def load_configmap(self):
        with open(CONFIG_MAP_FILE, "r") as f:
            deployment = yaml.safe_load(f)["deployment"]
            self.deployment_name = deployment.get("name", "")
            lora_adapters = deployment["models"]
            self.host, self.port = (
                deployment.get("host") or "localhost",
                deployment.get("port") or "8000",
            )
            self.config_map_adapters = {adapter["id"]:adapter for adapter in lora_adapters}

    def get_registered_adapters(self):
        """Retrieves all loaded models on server"""
        url = f"http://{self.host}:{self.port}/v1/models"
        try:
            response = requests.get(url)
            adapters = {adapter["id"]: adapter for adapter in response.json()["data"]}
            self.registered_adapters = adapters
        except requests.exceptions.RequestException as e:
            logging.error(f"Error communicating with vLLM server: {e}")

    def reconcile(self):
        """Reconciles model server with current version of configmap"""
        self.get_registered_adapters()
        self.load_configmap()
        for adapter_id, lora_adapter in self.config_map_adapters.items():
            logging.info(f"Processing adapter {adapter_id}")
            if lora_adapter.get("toRemove"):
                e = self.unload_adapter(lora_adapter)
                lora_adapter["timestamp"] = current_time_human()
                lora_adapter["status"] = {
                    "timestamp": current_time_human(),
                    "operation": "unload",
                    "errors": [e],
                }
            else:
                e = self.load_adapter(lora_adapter)
                lora_adapter["status"] = {
                    "timestamp": current_time_human(),
                    "operation": "load",
                    "errors": [e],
                }
        self.update_status_config()

    def update_status_config(self):
        models = list(self.config_map_adapters.values())
        deployment = {
            "name": self.deployment_name,
            "host": self.host,
            "port": self.port,
            "models": models,
        }
        config = {"deployment":deployment}
        with open(CONFIG_MAP_FILE, "w") as f:
            yaml.dump(config, f, indent=2)

    def load_adapter(self, adapter):
        """Sends a request to load the specified model."""
        adapter_id = adapter["id"]
        if adapter_id in self.registered_adapters or adapter.get("toRemove"):
            return "already loaded"
        url = f"http://{self.host}:{self.port}/v1/load_lora_adapter"
        payload = {
            "lora_name": adapter_id,
            "lora_path": adapter["source"],
            "base_model_name": adapter["base-model"],
        }
        try:
            response = requests.post(url, json=payload)
            response.raise_for_status()
            logging.info(f"Loaded model {adapter_id}")
            self.get_registered_adapters()
            return ""
        except requests.exceptions.RequestException as e:
            logging.error(f"Error loading model {adapter_id}: {e}")
            return f"Error loading model {adapter_id}: {e}"

    def unload_adapter(self, adapter):
        """Sends a request to unload the specified model."""
        adapter_id = adapter["id"]
        if adapter_id not in self.registered_adapters:
            return "already unloaded"
        url = f"http://{self.host}:{self.port}/v1/unload_lora_adapter"
        payload = {"lora_name": adapter_id}
        try:
            response = requests.post(url, json=payload)
            response.raise_for_status()
            logging.info(f"Unloaded model {adapter_id}")
            self.get_registered_adapters()
            return None
        except requests.exceptions.RequestException as e:
            logging.error(f"Error unloading model {adapter_id}: {e}")
            return f"Error unloading model {adapter_id}: {e}"


def main():
    """Loads the target configuration, compares it with the server's models,
    and loads/unloads models accordingly."""

    reconcilerInstance = LoraReconciler()
    reconcilerInstance.reconcile()
    observer = Observer()
    event_handler = ConfigWatcher(reconcilerInstance.reconcile)
    observer.schedule(event_handler, path=CONFIG_MAP_FILE, recursive=False)
    observer.start()
    try:
        while True:
            time.sleep(1)
    except KeyboardInterrupt:
        logging.info("Lora Adapter Dynamic configuration Reconciler stopped")
        observer.stop()
    observer.join()


if __name__ == "__main__":
    main()
