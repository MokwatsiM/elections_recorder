#!/usr/bin/env python3
"""Compare our API with the IEC's files for a random sample of areas at every level.

The IEC regenerates files every few minutes, so a mismatch only counts when the
two sides report the same ReportDate. Usage: accuracy-check.py [API base] [samples]
"""
import json, random, subprocess, sys

API = sys.argv[1] if len(sys.argv) > 1 else "http://localhost:8080/api/v1"
SAMPLES = int(sys.argv[2]) if len(sys.argv) > 2 else 12
IEC = "https://results.elections.org.za/dashboards/lge/MapsJason/2026/EEID1887"
checked = skipped = 0
mismatches = []


def get(url):
    # curl rather than urllib, whose CA certificates depend on how Python was installed.
    out = subprocess.run(["curl", "-sSf", "--compressed", "--max-time", "60", "-A", "LGE-Results-Dashboard/accuracy-check", url],
                         check=True, capture_output=True).stdout
    return json.loads(out.decode("utf-8-sig"))


def same(label, ours, theirs):
    global checked
    checked += 1
    if ours != theirs:
        mismatches.append(f"{label}: ours={ours} iec={theirs}")


def parties(label, ours, theirs):
    theirs = {p["ID"]: p for p in theirs if p["TotalValidVotes"] or p.get("TotalSeatsWon")}
    ours = {p["id"]: p for p in ours}
    same(f"{label} party set", set(ours), set(theirs))
    for pid in set(ours) & set(theirs):
        o, t = ours[pid], theirs[pid]
        same(f"{label} {pid} votes", o["votes"], t["TotalValidVotes"])
        same(f"{label} {pid} pct", o["pct"], t["PercOfVotes"])
        same(f"{label} {pid} seats", o.get("seats", 0), t.get("TotalSeatsWon", 0))


def area(label, ours, t):
    p = ours["progress"]
    same(f"{label} vds", p["vds"], t["VDsComplete"])
    same(f"{label} wards", p["wards"], t["WardsComplete"])
    same(f"{label} seats", p["seats"], t["SeatsComplete"])
    parties(label, ours["parties"], t["PartyBallotResults"])


def date_key(s):
    return s[:19]


def compare(label, ours, t):
    global skipped
    if date_key(ours["reportDate"]) != date_key(t["ReportDate"]):
        skipped += 1  # one side has moved on to a newer generation
        return False
    area(label, ours, t)
    return True


compare("national", get(f"{API}/national"), get(f"{IEC}LiteMapsNational.js"))
for pid in random.sample(range(1, 10), 3):
    compare(f"province {pid}", get(f"{API}/provinces/{pid}"), get(f"{IEC}Province{pid}.js"))

rows = get(f"{API}/map/municipality")
for m in random.sample(rows, min(SAMPLES, len(rows))):
    mid = m["id"]
    ours, t = get(f"{API}/municipalities/{mid}"), get(f"{IEC}Munic{mid}.js")
    if not compare(f"municipality {mid}", ours, t):
        continue
    reported = [w for w in t["WardResults"] if any(p["TotalValidVotes"] for p in w["PartyBallotResults"])]
    for w in random.sample(reported, min(3, len(reported))):
        ow = get(f"{API}/municipalities/{mid}/wards/{w['WardID']}")
        parties(f"ward {w['WardID']}", ow["parties"], w["PartyBallotResults"])
        vds = [v for v in w["VotingDistrictResults"] if any(p["TotalValidVotes"] for p in v["PartyBallotResults"])]
        for v in random.sample(vds, min(2, len(vds))):
            ov = get(f"{API}/municipalities/{mid}/vds/{v['VotingDistrictID']}")
            parties(f"vd {v['VotingDistrictID']}", ov["parties"], v["PartyBallotResults"])

print(f"compared {checked} values, skipped {skipped} areas where the IEC had moved on")
if mismatches:
    print(f"{len(mismatches)} MISMATCHES:")
    print("\n".join(mismatches[:40]))
    sys.exit(1)
print("all values match")
