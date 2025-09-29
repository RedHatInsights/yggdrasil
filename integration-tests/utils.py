"""
This module contains utilities for yggdrasil integration tests.
"""

import subprocess
import time
import logging
import os
import tomllib

logger = logging.getLogger(__name__)

# Path to yggdrasil client ID file
YGGDRASIL_CLIENT_ID_PATH = "/var/lib/yggdrasil/client-id"


def get_yggdrasil_client_id():
    """
    Try to get yggdrasil client ID used in MQTT messages
    :return: client ID string
    """
    logger.info("Getting yggdrasil client ID")
    try:
        with open(YGGDRASIL_CLIENT_ID_PATH, "r") as client_id_file:
            client_id = client_id_file.read()
    except IOError as err:
        logger.error(f"unable to read '{YGGDRASIL_CLIENT_ID_PATH}': {err}")
        client_id = None
    return client_id

def loop_until(function, assertation, poll_sec=1, timeout_sec=10):
    """
    The helper function to handle a time period waiting for an external service
    to update its state. The function can return arbitrary object and assertation
    function has to be able to check validity of this returned object

    an example:

       assert loop_until(function=is_echo_worker_running, assertation=lambda res: res == True)

    The loop function will retry to run function every poll_sec
    until the total time exceeds timeout_sec.
    """
    start = time.time()
    result = False
    while result is False and (time.time() - start < timeout_sec):
        time.sleep(poll_sec)
        function_result = function()
        result = assertation(function_result)
    return result



def get_yggdrasil_config_details(config_path="/etc/yggdrasil/config.toml"):
    """
    Get server host, port, path-prefix from yggdrasil config.toml.

    Returns:
        tuple: (server_host, server_port, path_prefix)
    """
    if not os.path.exists(config_path):
        raise FileNotFoundError(f"Config file not found: {config_path}")

    with open(config_path, "rb") as f:
        config = tomllib.load(f)

    servers = config.get("server", [])
    if not servers:
        raise ValueError("No 'server' entry found in config.toml")

    # Parse the first server entry (e.g., tcp://localhost:1883, mqtt://example.com:8883, etc.)
    server_url = servers[0]
    
    # Remove protocol prefix if present
    if "://" in server_url:
        protocol, server_url = server_url.split("://", 1)
    
    # Split host and port
    if ":" in server_url:
        host, port = server_url.rsplit(":", 1)  # rsplit to handle IPv6 addresses
    else:
        raise ValueError(f"No port specified in server URL: {servers[0]}")
    path_prefix = config.get("path-prefix", "yggdrasil")
    return host, int(port), path_prefix

def yggdrasil_service_is_active():
    """
    Check if yggdrasil service is active/running
    :return: Return True when yggdrasil service is active
    """
    proc = subprocess.run(
        ["systemctl", "is-active", "yggdrasil"],
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        universal_newlines=True,
    )
    return proc.returncode == 0 and proc.stdout.strip() == "active"