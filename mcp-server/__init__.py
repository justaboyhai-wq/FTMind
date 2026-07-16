#!/usr/bin/env python3
"""
Keystone MCP Server Package

A Model Context Protocol server that provides access to the Keystone knowledge management API.
"""

__version__ = "1.0.0"
__author__ = "Keystone Team"
__description__ = "Keystone MCP Server - Model Context Protocol server for Keystone API"

from .keystone_mcp_server import KeystoneClient, run

__all__ = ["KeystoneClient", "run"]
