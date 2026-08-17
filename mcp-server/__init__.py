#!/usr/bin/env python3
"""
FMind MCP Server Package

A Model Context Protocol server that provides access to the FMind knowledge management API.
"""

__version__ = "1.0.0"
__author__ = "FMind Team"
__description__ = "FMind MCP Server - Model Context Protocol server for FMind API"

from .fmind_mcp_server import FMindClient, run

__all__ = ["FMindClient", "run"]
