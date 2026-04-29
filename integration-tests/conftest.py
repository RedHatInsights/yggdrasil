def pytest_configure(config):
    config.addinivalue_line("markers", "tier1: tier 1 integration tests")
