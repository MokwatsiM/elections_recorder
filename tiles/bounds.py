#!/usr/bin/env python3
"""Writes the bounding boxes the map uses to zoom to an area."""
import json, os, sys

here = os.path.dirname(os.path.abspath(__file__))
public = os.path.join(here, "..", "frontend", "public")


def bbox(geom):
    xs, ys = [], []
    def walk(c):
        if isinstance(c[0], (int, float)):
            xs.append(c[0]); ys.append(c[1])
        else:
            for x in c: walk(x)
    walk(geom["coordinates"])
    return [round(min(xs), 4), round(min(ys), 4), round(max(xs), 4), round(max(ys), 4)]


def boxes(name):
    with open(os.path.join(here, "raw", f"{name}.geojson"), encoding="utf-8") as f:
        fc = json.load(f)
    out = {}
    for feat in fc["features"]:
        b = bbox(feat["geometry"])
        i = str(feat["properties"]["id"])
        if i in out:
            a = out[i]; b = [min(a[0], b[0]), min(a[1], b[1]), max(a[2], b[2]), max(a[3], b[3])]
        out[i] = b
    return out


json.dump({"province": boxes("provinces"), "municipality": boxes("municipalities")}, open(os.path.join(public, "bounds.json"), "w"), separators=(",", ":"))
json.dump(boxes("wards"), open(os.path.join(public, "bounds-wards.json"), "w"), separators=(",", ":"))
print("bounds written")
