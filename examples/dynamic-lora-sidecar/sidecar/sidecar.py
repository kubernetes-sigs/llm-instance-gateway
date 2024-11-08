import requests
import yaml
import time
from watchfiles import awatch
from dataclasses import dataclass
import asyncio
import logging
import datetime
import os

CONFIG_MAP_FILE = os.environ.get("DYNAMIC_LORA_ROLLOUT_CONFIG", "/config/configmap.yaml")
BASE_FIELD = "vLLMLoRAConfig"
logging.basicConfig(
    level=logging.INFO, format="%(asctime)s - %(levelname)s - %(filename)s:%(lineno)d -  %(message)s",
    datefmt='%Y-%m-%d %H:%M:%S',
    handlers=[logging.StreamHandler()]
)
logging.Formatter.converter = time.localtime 


def current_time_human() -> str:
    now = datetime.datetime.now(datetime.timezone.utc).astimezone()
    return now.strftime("%Y-%m-%d %H:%M:%S %Z%z")

@dataclass
class LoraAdapter:
    """Class representation of lora adapters in config"""
    def __init__(self, id, source="", base_model=""):
        self.id = id
        self.source = source
        self.base_model = base_model

    def __eq__(self, other):
        return self.id == other.id

    def __hash__(self):
        return hash(self.id)


class LoraReconciler:
    """
    Reconciles adapters registered on vllm server with adapters listed in configmap in current state
    """

    def __init__(self):
        self.health_check_timeout = datetime.timedelta(seconds=300)
        self.health_check_interval = datetime.timedelta(seconds=15)

    @property
    def config(self):
        """Load configmap into memory"""
        try:
            with open(CONFIG_MAP_FILE, "r") as f:
                c = yaml.safe_load(f)
                if c is None:
                    c = {}
                c = c.get("vLLMLoRAConfig",{})
                config_name = c.get("name","")
                logging.info(f"loaded vLLMLoRAConfig {config_name} from {CONFIG_MAP_FILE}")
                return c
        except Exception as e:
            logging.error(f"cannot load config {CONFIG_MAP_FILE} {e}")
            return {}
    
    @property
    def host(self):
        """Model server host"""
        return self.config.get("host", "localhost")

    @property
    def port(self):
        """Model server port"""
        return self.config.get("port", 8000)
    
    @property
    def model_server(self):
        """Model server {host}:{port}"""
        return f"{self.host}:{self.port}"

    @property
    def ensure_exist_adapters(self):
        """Lora adapters in config under key `ensureExist` in set"""
        adapters = self.config.get("ensureExist", {}).get("models", set())
        return set([LoraAdapter(adapter["id"], adapter["source"], adapter.get("base-model","")) for adapter in adapters])

    @property
    def ensure_not_exist_adapters(self):
        """Lora adapters in config under key `ensureNotExist` in set"""
        adapters = self.config.get("ensureNotExist", {}).get("models", set())
        return set([LoraAdapter(adapter["id"], adapter["source"], adapter.get("base-model","")) for adapter in adapters])

    @property
    def registered_adapters(self):
        """Lora Adapters registered on model server"""
        url = f"http://{self.model_server}/v1/models"
        if not self.is_server_healthy:
            logging.error(f"vllm server at {self.model_server} not healthy")
            return set()
        try:
            response = requests.get(url)
            adapters = [
                LoraAdapter(a.get("id", ""), a.get("")) for a in response.json()["data"]
            ]
            return set(adapters)
        except requests.exceptions.RequestException as e:
            logging.error(f"Error communicating with vLLM server: {e}")
            return set()

    @property
    def is_server_healthy(self) -> bool:
        """probe server's health endpoint until timeout or success"""
        
        def check_health() -> bool:
            """Checks server health"""
            url = f"http://{self.model_server}/health"
            try:
                response = requests.get(url)
                return response.status_code == 200
            except requests.exceptions.RequestException:
                return False
        
        start_time = datetime.datetime.now()
        while datetime.datetime.now() - start_time < self.health_check_timeout:
            if check_health():
                return True
            time.sleep(self.health_check_interval.seconds)
        return False
    
    def load_adapter(self, adapter: LoraAdapter):
        """Sends a request to load the specified model."""
        if adapter in self.registered_adapters:
            logging.info(f"{adapter.id} already present on model server {self.model_server}")
            return
        url = f"http://{self.model_server}/v1/load_lora_adapter"
        payload = {
            "lora_name": adapter.id,
            "lora_path": adapter.source,
            "base_model_name": adapter.base_model
        }
        try:
            response = requests.post(url, json=payload)
            response.raise_for_status()
            logging.info(f"loaded model {adapter.id}")
        except requests.exceptions.RequestException as e:
            logging.error(f"error loading model {adapter.id}: {e}")

    def unload_adapter(self, adapter: LoraAdapter):
        """Sends a request to unload the specified model."""
        if adapter not in self.registered_adapters:
            logging.info(f"{adapter.id} already doesn't exist on model server {self.model_server}")
            return
        url = f"http://{self.model_server}/v1/unload_lora_adapter"
        payload = {"lora_name": adapter.id}
        try:
            response = requests.post(url, json=payload)
            response.raise_for_status()
            logging.info(f"unloaded model {adapter.id}")
            return None
        except requests.exceptions.RequestException as e:
            logging.error(f"error unloading model {adapter.id}: {e}")
            return f"error unloading model {adapter.id}: {e}"

    def reconcile(self):
        """Reconciles model server with current version of configmap"""
        logging.info(f"reconciling model server {self.model_server} with config stored at {CONFIG_MAP_FILE}")
        if not self.is_server_healthy:
            logging.error(f"vllm server at {self.model_server} not healthy")
            return
        invalid_adapters = ", ".join(str(a.id) for a in self.ensure_exist_adapters & self.ensure_not_exist_adapters)
        logging.warning(f"skipped adapters found in both `ensureExist` and `ensureNotExist` {invalid_adapters}")
        adapters_to_load = self.ensure_exist_adapters - self.ensure_not_exist_adapters
        adapters_to_load_id = ", ".join(str(a.id) for a in adapters_to_load)
        logging.info(f"adapter to load {adapters_to_load_id}")
        for adapter in adapters_to_load:
            self.load_adapter(adapter)
        adapters_to_unload = self.ensure_not_exist_adapters - self.ensure_exist_adapters
        adapters_to_unload_id = ", ".join(str(a.id) for a in adapters_to_unload)
        logging.info(f"adapters to unload {adapters_to_unload_id}")
        for adapter in adapters_to_unload:
            self.unload_adapter(adapter)



async def main():
    """Loads the target configuration, compares it with the server's models,
    and loads/unloads models accordingly."""

    reconcilerInstance = LoraReconciler()
    logging.info(f"running reconcile for initial loading of configmap {CONFIG_MAP_FILE}")
    reconcilerInstance.reconcile()
    logging.info(f"beginning watching of configmap {CONFIG_MAP_FILE}")
    async for _ in awatch('/config/configmap.yaml'):
        logging.info(f"Config '{CONFIG_MAP_FILE}' modified!'" )
        reconcilerInstance.reconcile()


if __name__ == "__main__":
    asyncio.run(main())
