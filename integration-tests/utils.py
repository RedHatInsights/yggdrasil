"""
This module contains utilities for yggdrasil integration tests.
"""

import time

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
