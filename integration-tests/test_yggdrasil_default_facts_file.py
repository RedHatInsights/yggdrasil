import subprocess
import json
import time
import pytest
import paho.mqtt.client as mqtt
from utils import (
    yggdrasil_service_is_active,
    get_yggdrasil_config_details,
    get_yggdrasil_client_id,
)


def test_yggdrasil_publishes_canonical_facts(external_candlepin, rhc, test_config):
    """
    :title: Verify Yggdrasil publishes canonical facts using default facts file.
    :description:
        This test verifies that Yggdrasil publishes canonical facts using the default 
facts file when the facts-file configuration is not defined in the config.toml file.
    :steps:
        1.  Comment out any existing facts-file configuration in /etc/yggdrasil/config.toml.
        2.  Connect the system using 'rhc connect' to ensure facts are available.
        3.  Verify that RHC reports being registered.
        4.  Verify that the yggdrasil service is active.
        5.  Configure MQTT server to localhost and restart yggdrasil service.
        6.  Verify canonical_facts got published on MQTT topics.
    :expectedresults:
        1.  The facts-file configuration is successfully commented out.
        2.  The 'rhc connect' command executes successfully.
        3.  RHC indicates the system is registered.
        4.  The yggdrasil service is in an active state.
        5.  MQTT configuration is updated and yggdrasil service restarts successfully.
        6.  Canonical facts are published and received on the expected MQTT topic.
    """

    try:
        # Ensure facts-file is not defined in config (comment it out if it exists)
        subprocess.run(
            ["sed", "-i", r"s/^facts-file/#facts-file/", "/etc/yggdrasil/config.toml"],
            check=False,
        )

        # run rhc connect so that some facts are available for testing
        rhc.connect(
            username=test_config.get("candlepin.username"),
            password=test_config.get("candlepin.password"),
        )
        # validate the connection
        assert rhc.is_registered
        assert yggdrasil_service_is_active()

        # Verify canonical_facts were published to MQTT
        assert wait_for_canonical_facts(), "No canonical_facts published on MQTT topics"

    finally:
        # restore the original facts-file configuration in config.toml
        subprocess.run(
            ["sed", "-i", r"s/^#facts-file/facts-file/", "/etc/yggdrasil/config.toml"],
            check=False,
        )


def wait_for_canonical_facts(timeout: int = 30) -> dict:
    """
    Wait for canonical_facts in a connection-status message from yggdrasil MQTT broker.

    Args:
        timeout (int): seconds to wait before giving up.

    Returns:
        dict: canonical_facts if found, else None
    """
    canonical_facts = {}

    def on_message(client, userdata, msg):
        nonlocal canonical_facts
        try:
            payload = json.loads(msg.payload.decode())
            if payload.get("type") == "connection-status":
                facts = payload.get("content", {}).get("canonical_facts")
                if facts:
                    canonical_facts = facts
                    client.disconnect()
        except Exception as e:
            print(f"Error parsing message: {e}")

    # set mqtt server url to localhost for yggdrasil service so that we can subscribe to topic and
    # receive facts locally
    subprocess.run(
        [
            "sed",
            "-i",
            r"/^#\?server\s*=/d; 1a server = [\"mqtt://localhost:1883\"]",
            "/etc/yggdrasil/config.toml",
        ],
        check=False,
    )
    host, port, path_prefix = get_yggdrasil_config_details()
    client_id = get_yggdrasil_client_id()
    if not client_id:
        raise RuntimeError("Unable to determine yggdrasil client ID")

    topic = f"{path_prefix}/{client_id}/control/out"

    client = mqtt.Client()
    client.on_message = on_message
    client.connect(host, port, 60)
    client.subscribe(topic)

    # Restart yggdrasil service to pick up new configuration and publish fresh facts
    subprocess.run(["systemctl", "restart", "yggdrasil"], check=False)

    start = time.time()
    while not canonical_facts and (time.time() - start) < timeout:
        client.loop(timeout=1.0)

    return canonical_facts if canonical_facts else None
