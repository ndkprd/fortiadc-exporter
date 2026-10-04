"""Mask identifying values in a FortiADC JSON response (stdin -> stdout).

Keeps shapes and numbers; lists are cut to 3 entries (one null placeholder kept).
"""
import json
import re
import sys

NAME_KEYS = {"mkey", "host-name", "host-domain", "domain-name", "pool", "hostmkey",
             "hostName", "address", "local_sn", "serialNumber", "local_address", "loginId"}
IPV4 = re.compile(r"\b\d{1,3}(?:\.\d{1,3}){3}\b")
seen = {}


def mask_name(key, value):
    return seen.setdefault((key, value), f"{key}-{len(seen) + 1}")


def walk(o, key=None):
    if isinstance(o, dict):
        return {k: walk(v, k) for k, v in o.items()}
    if isinstance(o, list):
        kept = [x for x in o if x is not None][:3] + ([None] if None in o else [])
        return [walk(v, key) for v in kept]
    if isinstance(o, str):
        if key in NAME_KEYS and o not in ("", "N/A"):
            return mask_name(key, o)
        return IPV4.sub("192.0.2.1", o)
    return o


json.dump(walk(json.load(sys.stdin)), sys.stdout, indent=2)
