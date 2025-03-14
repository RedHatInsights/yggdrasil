"""
This Python module contains integration tests for yggdrasil service.
"""

import subprocess
import logging

logger = logging.getLogger(__name__)

def test_status_yggdrasil_service():
    """
    This test tries to get status of yggdrasil service
    """
    logger.info("Getting status of yggdrasil service")
    proc = subprocess.run(
        ["systemctl", "status", "yggdrasil"],
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        universal_newlines=True,
    )
    assert "yggdrasil system service" in proc.stdout

def test_start_yggdrasil_service():
    """
    This test tries to start yggdrasil service
    """
    logger.info("Stopping yggdrasil service")
    proc = subprocess.run(
        ["systemctl", "stop", "yggdrasil"],
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        universal_newlines=True,
    )
    # Check if the yggdrasil was stopped
    assert proc.returncode == 0

    logger.info("Starting yggdrasil service")
    proc = subprocess.run(
        ["systemctl", "start", "yggdrasil"],
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        universal_newlines=True,
    )
    # Check if the yggdrasil was started
    assert proc.returncode == 0

    proc = subprocess.run(
        ["systemctl", "is-active", "yggdrasil"],
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        universal_newlines=True,
    )
    assert proc.returncode == 0
    assert proc.stdout.strip() == "active"
    logger.info("The yggdrasil service was started")

def test_stop_yggdrasil_service():
    """
    This test tries to start and then stop yggdrasil service.
    Yeah, it is necessary to start the service first. Then it
    is possible to test stopping it.
    """
    logger.info("Starting yggdrasil service")
    proc = subprocess.run(
        ["systemctl", "start", "yggdrasil"],
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        universal_newlines=True,
    )
    # Check if the yggdrasil was started
    assert proc.returncode == 0

    proc = subprocess.run(
        ["systemctl", "is-active", "yggdrasil"],
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        universal_newlines=True,
    )
    assert proc.returncode == 0
    assert proc.stdout.strip() == "active"
    logger.info("The yggdrasil service was started")

    logger.info("Stopping yggdrasil service")
    proc = subprocess.run(
        ["systemctl", "stop", "yggdrasil"],
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        universal_newlines=True,
    )
    # Check if the yggdrasil was stopped
    assert proc.returncode == 0
    proc = subprocess.run(
        ["systemctl", "is-active", "yggdrasil"],
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        universal_newlines=True,
    )
    assert proc.returncode == 3
    assert proc.stdout.strip() == "inactive"
    logger.info("The yggdrasil service was stopped")
