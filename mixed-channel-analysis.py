#!/usr/bin/env python3
"""
Compute optimal fixed channel sets for mixed analog + DJI FPV sessions.
Maximizes minimum spacing and minimizes IMD interference.
"""

from itertools import combinations
from collections import defaultdict

# Channel pools
RACEBAND = {
    "R1": 5658, "R2": 5695, "R3": 5732, "R4": 5769,
    "R5": 5806, "R6": 5843, "R7": 5880, "R8": 5917,
}

DJI_FCC_20 = {
    "O-CH1": 5669, "O-CH2": 5705, "O-CH3": 5769, "O-CH4": 5805,
    "O-CH5": 5840, "O-CH6": 5876, "O-CH7": 5912,
}

# Guard band to power mapping
GUARD_TO_POWER = {
    10: 25, 12: 100, 14: 200, 16: 400, 24: 600, 28: 800, 32: 1000,
}
GUARD_BANDS = sorted(GUARD_TO_POWER.keys())

# All channels are 20 MHz bandwidth
BW = 20
DEFAULT_GUARD = 10


def required_spacing(guard_band=DEFAULT_GUARD):
    """Required spacing between two 20 MHz channels."""
    return (BW / 2) + (BW / 2) + guard_band


def min_spacing(freqs):
    """Calculate minimum spacing between any two frequencies in the set."""
    sorted_f = sorted(freqs)
    if len(sorted_f) < 2:
        return float('inf')
    return min(sorted_f[i+1] - sorted_f[i] for i in range(len(sorted_f) - 1))


def imd_score(freqs):
    """
    Calculate IMD score using proximity-weighted formula.
    Third-order IMD: F_imd = 2*F1 - F2 for all pairs.
    Penalty for each IMD product within 20 MHz of an active channel.
    """
    freq_list = sorted(freqs)
    n = len(freq_list)
    if n < 2:
        return 100.0

    # Generate all third-order IMD products
    imd_products = []
    for i in range(n):
        for j in range(n):
            if i != j:
                imd = 2 * freq_list[i] - freq_list[j]
                imd_products.append(imd)

    # Calculate penalty
    total_penalty = 0.0
    for imd in imd_products:
        for f in freq_list:
            distance = abs(imd - f)
            if distance > 0 and distance < 20:  # within 20 MHz but not exactly on the channel
                total_penalty += (20 - distance) ** 2

    score = max(0.0, 100.0 - total_penalty / (5.0 * n))
    return round(score, 1)


def max_power_tolerance(freqs):
    """
    What's the highest guard band that still fits the minimum spacing?
    required_spacing = BW/2 + BW/2 + guard = 20 + guard
    So max_guard = min_spacing - 20
    """
    ms = min_spacing(freqs)
    max_guard = ms - BW  # BW/2 + BW/2 = BW = 20
    # Find highest guard band that fits
    best_power = 0
    best_guard = 0
    for gb in GUARD_BANDS:
        if gb <= max_guard:
            best_power = GUARD_TO_POWER[gb]
            best_guard = gb
    return best_power, best_guard


def find_best_sets(analog_count, dji_count, top_n=3):
    """
    Find the best channel sets with the given number of analog and DJI channels.
    Returns list of (channels_dict, min_spacing, imd_score, power_mw, guard_band).
    channels_dict maps channel_name -> (freq, type).
    """
    analog_names = list(RACEBAND.keys())
    dji_names = list(DJI_FCC_20.keys())

    results = []

    for analog_combo in combinations(analog_names, analog_count):
        for dji_combo in combinations(dji_names, dji_count):
            # Build channel set
            channels = {}
            for name in analog_combo:
                channels[name] = (RACEBAND[name], "analog")
            for name in dji_combo:
                channels[name] = (DJI_FCC_20[name], "DJI")

            freqs = [v[0] for v in channels.values()]

            # Check for frequency collisions (same freq used twice)
            if len(set(freqs)) < len(freqs):
                continue  # Skip if two channels land on same frequency

            ms = min_spacing(freqs)
            imd = imd_score(freqs)
            power_mw, guard = max_power_tolerance(freqs)

            results.append((channels, ms, imd, power_mw, guard))

    # Sort by min_spacing desc, then IMD score desc
    results.sort(key=lambda x: (x[1], x[2]), reverse=True)
    return results[:top_n]


def format_channels(channels):
    """Format channel dict into a readable string."""
    items = sorted(channels.items(), key=lambda x: x[1][0])  # sort by freq
    parts = []
    for name, (freq, typ) in items:
        marker = "[D]" if typ == "DJI" else "[A]"
        parts.append(f"{name}={freq}{marker}")
    return ", ".join(parts)


def print_results(pilot_count, analog_count, dji_count, results):
    """Print results for a specific configuration."""
    label = f"{pilot_count} pilots: {analog_count}A + {dji_count}D"
    print(f"\n{'='*80}")
    print(f"  {label}")
    print(f"{'='*80}")

    if not results:
        print("  No valid combinations found.")
        return

    for rank, (channels, ms, imd, power_mw, guard) in enumerate(results, 1):
        print(f"\n  #{rank}  Min Spacing: {ms} MHz | IMD Score: {imd} | "
              f"Max Power: {power_mw} mW (guard={guard} MHz)")
        print(f"       {format_channels(channels)}")


def main():
    print("=" * 80)
    print("  MIXED ANALOG + DJI CHANNEL OPTIMIZATION")
    print("  All channels 20 MHz bandwidth, default guard band 10 MHz")
    print("  Required spacing = 30 MHz minimum (at 25 mW)")
    print("=" * 80)

    # Define the configurations to analyze
    configs = [
        # (total_pilots, analog_count, dji_count)
        # 2 pilots
        (2, 2, 0),
        (2, 1, 1),
        (2, 0, 2),
        # 3 pilots
        (3, 3, 0),
        (3, 2, 1),
        (3, 1, 2),
        (3, 0, 3),
        # 4 pilots (focus)
        (4, 4, 0),
        (4, 3, 1),
        (4, 2, 2),
        (4, 0, 4),
        # 5 pilots (focus)
        (5, 5, 0),
        (5, 4, 1),
        (5, 3, 2),
        (5, 0, 5),
        # 6 pilots (focus)
        (6, 6, 0),
        (6, 5, 1),
        (6, 4, 2),
        (6, 3, 3),
        (6, 0, 6),
        # 7 pilots
        (7, 7, 0),
        (7, 6, 1),
        (7, 5, 2),
        (7, 4, 3),
        (7, 0, 7),
        # 8 pilots
        (8, 8, 0),
        (8, 7, 1),
        (8, 6, 2),
        (8, 5, 3),
    ]

    for total, analog, dji in configs:
        assert analog + dji == total
        results = find_best_sets(analog, dji, top_n=3)
        print_results(total, analog, dji, results)

    # Summary table
    print("\n\n")
    print("=" * 120)
    print("  SUMMARY TABLE — Best set per configuration")
    print("=" * 120)
    print(f"  {'Config':<16} {'Min Spacing':>11} {'IMD Score':>9} {'Max Power':>9} {'Channels'}")
    print(f"  {'-'*15} {'-'*11} {'-'*9} {'-'*9} {'-'*60}")

    for total, analog, dji in configs:
        results = find_best_sets(analog, dji, top_n=1)
        if results:
            channels, ms, imd, power_mw, guard = results[0]
            label = f"{total}p: {analog}A+{dji}D"
            ch_str = format_channels(channels)
            print(f"  {label:<16} {ms:>8} MHz {imd:>9} {power_mw:>6} mW  {ch_str}")
        else:
            label = f"{total}p: {analog}A+{dji}D"
            print(f"  {label:<16}  — no valid combinations —")

    # Near-overlap analysis
    print("\n\n")
    print("=" * 80)
    print("  NEAR-OVERLAP ANALYSIS: Analog-DJI channel proximity")
    print("=" * 80)
    print(f"  {'Analog':<8} {'Freq':>6}   {'DJI':<8} {'Freq':>6}   {'Gap':>5}")
    print(f"  {'-'*7} {'-'*6}   {'-'*7} {'-'*6}   {'-'*5}")

    for aname, afreq in sorted(RACEBAND.items(), key=lambda x: x[1]):
        for dname, dfreq in sorted(DJI_FCC_20.items(), key=lambda x: x[1]):
            gap = abs(afreq - dfreq)
            if gap <= 15:
                print(f"  {aname:<8} {afreq:>5}   {dname:<8} {dfreq:>5}   {gap:>4} MHz"
                      f"{'  *** OVERLAP' if gap == 0 else '  * CONFLICT' if gap < 10 else ''}")

    # Practical guidance section
    print("\n\n")
    print("=" * 80)
    print("  PRACTICAL GUIDANCE")
    print("=" * 80)
    print("""
  Key findings for mixed sessions:

  1. NEAR-OVERLAPPING PAIRS can share a "slot":
     R4/O-CH3 (same freq), R5/O-CH4 (1 MHz), R6/O-CH5 (3 MHz), R7/O-CH6 (4 MHz), R8/O-CH7 (5 MHz)
     These pairs effectively occupy the same spectral space.

  2. CONFLICTING PAIRS that waste spectrum:
     R2/O-CH2 (10 MHz) — too close to coexist, too far to share
     R1/O-CH1 (11 MHz) — same issue

  3. BEST STRATEGY for mixed groups:
     - Treat near-overlap pairs as single slots
     - Spread analog and DJI channels across the band
     - Avoid R1+O-CH1 and R2+O-CH2 together (wastes slots)
""")


if __name__ == "__main__":
    main()
