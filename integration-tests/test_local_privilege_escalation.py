import subprocess
import logging

logger = logging.getLogger(__name__)


def test_local_privilege_escalation():
    """
    :id: 09ca0098-6dcd-465b-9f10-262776b858b0
    :title: Verify unprivileged user cannot dispatch commands to workers
    :description:
        This test ensures that a non-root user cannot dispatch messages to com.redhat.Yggdrasil.Dispatch() DBus method.
        It switches to a non-root test user and verifies that dispatch fails with the 
        expected authorization error.
        In this test, we will send package installation command as non-privileged user 
        to package-manager worker.
        Ref Bug: https://issues.redhat.com/browse/RHEL-88585
    :steps:
        1. Create a non-privileged test user.
        2. Ensure a package (for example zsh) is not installed.
        3. Run yggctl dispatch as non-privileged user.
    :expectedresults:
        1. Test user exists.
        2. Test package is not installed initially.
        3. Package installation dispatch fails with authorization error (exit code 1).
    """
    test_user = "testuser_yggdrasil"
    package_name = "zsh" # Using zsh as a test package only.

    try:
        # Create a non-privileged user (if not exists)
        result = subprocess.run(["id", "-u", test_user], capture_output=True, text=True)
        if result.returncode != 0:
            logger.info(f"Creating {test_user} user")
            subprocess.run(["useradd", "--no-create-home", test_user], check=True)

        # Ensure test package is not installed (remove if present) to establish clean test state
        logger.info(f"Ensuring test package '{package_name}' is not installed ")
        result = subprocess.run(
            f"rpm -q {package_name}", shell=True, capture_output=True, text=True
        )
        if result.returncode == 0:  # package is installed
            subprocess.run(
                f"dnf remove -y {package_name} || yum remove -y {package_name}",
                shell=True,
                check=False,
            )

        # Attempt to dispatch package installation command as non-privileged user
        cmd = f"""echo '{{"command":"install","name":"{package_name}"}}' | yggctl dispatch --worker package_manager -"""
        logger.info(f"Attempting package installation as unprivileged user {test_user}: {cmd}")
        result_login = subprocess.run(
            ["su", "-", test_user, "-c", cmd],
            capture_output=True,
            text=True,
        )

        assert result_login.returncode != 0
        assert "sender is not authorized" in (
            result_login.stdout.lower() + result_login.stderr.lower()
        )

    finally:
        # Cleanup: remove the test user
        logger.info(f"Cleaning up user {test_user}")
        subprocess.run(["userdel", "-r", test_user], check=False)
