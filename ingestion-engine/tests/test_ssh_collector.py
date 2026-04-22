"""Unit tests for the SSH collector."""

import pytest

from app.collectors.base import Collector
from app.collectors.ssh import SSHCollector, parse_os_release


# ---------------------------------------------------------------------------
# Protocol / metadata tests
# ---------------------------------------------------------------------------


def test_ssh_collector_implements_protocol():
    """SSHCollector must satisfy the Collector protocol."""
    collector = SSHCollector()
    assert isinstance(collector, Collector)


def test_ssh_supported_fields():
    """supported_fields() must return the expected field names."""
    collector = SSHCollector()
    field_names = {f.field_name for f in collector.supported_fields()}
    expected = {
        "hostname",
        "serial_number",
        "vendor",
        "model",
        "os_type",
        "os_version",
        "cpu_cores",
        "memory_mb",
        "disk_gb",
        "ip_address",
        # sub_type distinguishes physical vs virtual hosts — set by the
        # ssh collector based on /sys/hypervisor presence or dmidecode output.
        "sub_type",
    }
    assert field_names == expected


# ---------------------------------------------------------------------------
# parse_os_release tests
# ---------------------------------------------------------------------------


def test_parse_os_release():
    """Ubuntu /etc/os-release parses to correct ID and VERSION_ID."""
    content = """\
NAME="Ubuntu"
VERSION="22.04.3 LTS (Jammy Jellyfish)"
ID=ubuntu
ID_LIKE=debian
PRETTY_NAME="Ubuntu 22.04.3 LTS"
VERSION_ID="22.04"
HOME_URL="https://www.ubuntu.com/"
SUPPORT_URL="https://help.ubuntu.com/"
BUG_REPORT_URL="https://bugs.launchpad.net/ubuntu/"
PRIVACY_POLICY_URL="https://www.ubuntu.com/legal/terms-and-policies/privacy-policy"
VERSION_CODENAME=jammy
UBUNTU_CODENAME=jammy
"""
    result = parse_os_release(content)
    assert result["os_type"] == "ubuntu"
    assert result["os_version"] == "22.04"


def test_parse_os_release_centos():
    """CentOS /etc/os-release parses to correct ID and VERSION_ID."""
    content = """\
NAME="CentOS Linux"
VERSION="7 (Core)"
ID="centos"
ID_LIKE="rhel fedora"
VERSION_ID="7"
PRETTY_NAME="CentOS Linux 7 (Core)"
ANSI_COLOR="0;31"
CPE_NAME="cpe:/o:centos:centos:7"
HOME_URL="https://www.centos.org/"
BUG_REPORT_URL="https://bugs.centos.org/"
"""
    result = parse_os_release(content)
    assert result["os_type"] == "centos"
    assert result["os_version"] == "7"


def test_parse_os_release_empty():
    """Empty / garbage input returns empty strings without raising."""
    assert parse_os_release("") == {"os_type": "", "os_version": ""}
    assert parse_os_release("this is not os-release content") == {
        "os_type": "",
        "os_version": "",
    }
    assert parse_os_release("# just a comment\n\n") == {
        "os_type": "",
        "os_version": "",
    }
