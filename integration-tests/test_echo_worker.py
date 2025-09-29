"""
This Python module contains integration tests for echo worker service.
It indirectly tests that yggdrasil service dispatch MQTT message to
"""

import subprocess
import logging
import time
from utils import loop_until, get_yggdrasil_client_id

logger = logging.getLogger(__name__)

# The scheme could be "mqtt://" or "mqtts://"
MQTT_SERVER_URL = "mqtt://localhost:1883"

ECHO_WORKER_DIRECTIVE = "echo"

ECHO_WORKER_SERVICE = "com.redhat.Yggdrasil1.Worker1.echo.service"

mqtt_message = """
"hello"
"""

def is_echo_worker_running():
    """
    Check if echo worker is running
    :return: Return true, when echo worker is running
    """
    proc = subprocess.run(
        ["systemctl", "is-active", ECHO_WORKER_SERVICE],
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        universal_newlines=True,
    )
    if proc.returncode == 0 and proc.stdout.strip() == "active":
        return True
    else:
        return False


def test_echo_started_on_mqtt_message():
    """
    Test that echo worker service is automatically started,
    when MQTT message is sent to yggdrasil service and this
    message is dispatched to echo worker.
    """
    logger.info("Starting yggdrasil service")
    proc = subprocess.run(
        ["systemctl", "start", "yggdrasil.service"],
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        universal_newlines=True,
    )
    # Check if the yggdrasil was started
    assert proc.returncode == 0

    client_id = get_yggdrasil_client_id()
    assert client_id is not None

    logger.info("Making sure that echo worker is stopped")
    proc = subprocess.run(
        ["systemctl", "stop", ECHO_WORKER_SERVICE],
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        universal_newlines=True,
    )
    assert proc.returncode == 0

    logger.info("Sending MQTT message to MQTT broker")
    logger.debug(f"yggd client ID: f{client_id}")

    proc = subprocess.run(
        [
            f"echo '{mqtt_message}'"
            "|"
            f"yggctl generate data-message --directive {ECHO_WORKER_DIRECTIVE} -"
            "|"
            f"mosquitto_pub --url {MQTT_SERVER_URL}/yggdrasil/{client_id}/data/in --stdin-line",
        ],
        shell=True,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        universal_newlines=True,
    )
    logger.debug(f"return code: {proc.returncode}")
    logger.debug(f"stdout: {proc.stdout}")
    logger.debug(f"stderr: {proc.stderr}")
    assert proc.returncode == 0

    time.sleep(1)

    # Check if the echo worker service was started
    result = loop_until(
        function=is_echo_worker_running,
        assertation=lambda res: res == True,
        poll_sec=0.2,
        timeout_sec=2
    )
    assert result is True

    logger.info("The echo service was started")
