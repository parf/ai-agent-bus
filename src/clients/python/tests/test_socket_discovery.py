"""Socket discovery never guesses a prerelease temporary endpoint."""
import os
import unittest
from types import SimpleNamespace
from unittest.mock import patch
from agent_bus.transport import discover_socket


class SocketDiscovery(unittest.TestCase):
    def test_missing_sockets_use_installed_endpoint(self):
        with patch.dict(os.environ, {'XDG_RUNTIME_DIR': '/fixture/runtime'}), \
             patch('agent_bus.transport.os.stat', side_effect=FileNotFoundError), \
             patch('agent_bus.transport.pwd.getpwuid', return_value=SimpleNamespace(pw_name='fixture')):
            self.assertEqual(discover_socket('token'), '/run/agent-bus/bus.sock')
            self.assertEqual(discover_socket(''), '/run/agent-bus/user-fixture.sock')
