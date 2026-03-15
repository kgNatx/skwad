#!/bin/bash
# Seed the usage dashboard with sample data.
# Run from the same directory as docker-compose.yaml.
# Usage: bash skwad/seed-usage-data.sh

CONTAINER="skwad"
DB="/data/skwad.db"

run() {
  docker exec "$CONTAINER" sqlite3 "$DB" "$1"
}

echo "Seeding usage data..."

# Austin, TX — 4 sessions
run "INSERT OR IGNORE INTO session_snapshots (session_code, created_at, expired_at, duration_minutes, peak_pilot_count, total_joins, rebalance_count, channel_change_count, video_systems, power_ceiling_mw, used_fixed_channels, city, region, country, latitude, longitude) VALUES ('SEED01', '2026-03-09T14:30:00', '2026-03-10T02:30:00', 720, 7, 8, 1, 3, '{\"dji_o3\":3,\"analog\":2,\"hdzero\":1,\"dji_o4\":1}', 200, 0, 'Austin', 'TX', 'US', 30.2672, -97.7431);"
run "INSERT OR IGNORE INTO session_snapshots (session_code, created_at, expired_at, duration_minutes, peak_pilot_count, total_joins, rebalance_count, channel_change_count, video_systems, power_ceiling_mw, used_fixed_channels, city, region, country, latitude, longitude) VALUES ('SEED02', '2026-03-11T10:15:00', '2026-03-11T22:15:00', 720, 5, 6, 0, 1, '{\"dji_o3\":2,\"analog\":2,\"hdzero\":1}', 400, 0, 'Austin', 'TX', 'US', 30.2672, -97.7431);"
run "INSERT OR IGNORE INTO session_snapshots (session_code, created_at, expired_at, duration_minutes, peak_pilot_count, total_joins, rebalance_count, channel_change_count, video_systems, power_ceiling_mw, used_fixed_channels, city, region, country, latitude, longitude) VALUES ('SEED03', '2026-03-12T09:00:00', '2026-03-12T21:00:00', 720, 9, 11, 2, 5, '{\"dji_o3\":3,\"dji_o4\":2,\"analog\":2,\"hdzero\":1,\"walksnail\":1}', 200, 1, 'Austin', 'TX', 'US', 30.2672, -97.7431);"
run "INSERT OR IGNORE INTO session_snapshots (session_code, created_at, expired_at, duration_minutes, peak_pilot_count, total_joins, rebalance_count, channel_change_count, video_systems, power_ceiling_mw, used_fixed_channels, city, region, country, latitude, longitude) VALUES ('SEED04', '2026-03-14T15:45:00', '2026-03-15T03:45:00', 720, 4, 4, 0, 0, '{\"dji_o3\":2,\"analog\":1,\"hdzero\":1}', 0, 0, 'Austin', 'TX', 'US', 30.2672, -97.7431);"

# Cedar Park, TX — 2 sessions
run "INSERT OR IGNORE INTO session_snapshots (session_code, created_at, expired_at, duration_minutes, peak_pilot_count, total_joins, rebalance_count, channel_change_count, video_systems, power_ceiling_mw, used_fixed_channels, city, region, country, latitude, longitude) VALUES ('SEED05', '2026-03-10T11:00:00', '2026-03-10T23:00:00', 720, 6, 7, 1, 2, '{\"dji_o3\":2,\"analog\":3,\"hdzero\":1}', 200, 0, 'Cedar Park', 'TX', 'US', 30.5052, -97.8203);"
run "INSERT OR IGNORE INTO session_snapshots (session_code, created_at, expired_at, duration_minutes, peak_pilot_count, total_joins, rebalance_count, channel_change_count, video_systems, power_ceiling_mw, used_fixed_channels, city, region, country, latitude, longitude) VALUES ('SEED06', '2026-03-13T14:00:00', '2026-03-14T02:00:00', 720, 5, 5, 0, 1, '{\"dji_o4\":2,\"analog\":2,\"walksnail\":1}', 400, 0, 'Cedar Park', 'TX', 'US', 30.5052, -97.8203);"

# San Marcos, TX — 1 session
run "INSERT OR IGNORE INTO session_snapshots (session_code, created_at, expired_at, duration_minutes, peak_pilot_count, total_joins, rebalance_count, channel_change_count, video_systems, power_ceiling_mw, used_fixed_channels, city, region, country, latitude, longitude) VALUES ('SEED07', '2026-03-11T09:30:00', '2026-03-11T21:30:00', 720, 8, 10, 1, 4, '{\"dji_o3\":3,\"analog\":3,\"hdzero\":2}', 200, 0, 'San Marcos', 'TX', 'US', 29.8833, -97.9414);"

# Round Rock, TX — 1 session
run "INSERT OR IGNORE INTO session_snapshots (session_code, created_at, expired_at, duration_minutes, peak_pilot_count, total_joins, rebalance_count, channel_change_count, video_systems, power_ceiling_mw, used_fixed_channels, city, region, country, latitude, longitude) VALUES ('SEED08', '2026-03-12T16:00:00', '2026-03-13T04:00:00', 720, 3, 3, 0, 0, '{\"dji_o3\":1,\"analog\":1,\"dji_o4\":1}', 0, 0, 'Round Rock', 'TX', 'US', 30.5083, -97.6789);"

# Knoxville, TN — 1 session
run "INSERT OR IGNORE INTO session_snapshots (session_code, created_at, expired_at, duration_minutes, peak_pilot_count, total_joins, rebalance_count, channel_change_count, video_systems, power_ceiling_mw, used_fixed_channels, city, region, country, latitude, longitude) VALUES ('SEED09', '2026-03-13T10:00:00', '2026-03-13T22:00:00', 720, 6, 7, 1, 2, '{\"dji_o3\":2,\"analog\":2,\"hdzero\":1,\"walksnail\":1}', 200, 0, 'Knoxville', 'TN', 'US', 35.9606, -83.9207);"

# Santa Fe, NM — 1 session
run "INSERT OR IGNORE INTO session_snapshots (session_code, created_at, expired_at, duration_minutes, peak_pilot_count, total_joins, rebalance_count, channel_change_count, video_systems, power_ceiling_mw, used_fixed_channels, city, region, country, latitude, longitude) VALUES ('SEED10', '2026-03-09T16:00:00', '2026-03-10T04:00:00', 720, 5, 6, 0, 1, '{\"dji_o3\":2,\"analog\":2,\"hdzero\":1}', 200, 0, 'Santa Fe', 'NM', 'US', 35.6870, -105.9378);"

# Los Angeles, CA — 2 sessions
run "INSERT OR IGNORE INTO session_snapshots (session_code, created_at, expired_at, duration_minutes, peak_pilot_count, total_joins, rebalance_count, channel_change_count, video_systems, power_ceiling_mw, used_fixed_channels, city, region, country, latitude, longitude) VALUES ('SEED11', '2026-03-10T13:00:00', '2026-03-11T01:00:00', 720, 8, 9, 1, 3, '{\"dji_o3\":3,\"dji_o4\":2,\"analog\":2,\"walksnail\":1}', 400, 0, 'Los Angeles', 'CA', 'US', 34.0522, -118.2437);"
run "INSERT OR IGNORE INTO session_snapshots (session_code, created_at, expired_at, duration_minutes, peak_pilot_count, total_joins, rebalance_count, channel_change_count, video_systems, power_ceiling_mw, used_fixed_channels, city, region, country, latitude, longitude) VALUES ('SEED12', '2026-03-13T11:00:00', '2026-03-13T23:00:00', 720, 6, 7, 0, 2, '{\"dji_o3\":2,\"analog\":3,\"hdzero\":1}', 200, 0, 'Los Angeles', 'CA', 'US', 34.0522, -118.2437);"

# Orlando, FL — 3 sessions
run "INSERT OR IGNORE INTO session_snapshots (session_code, created_at, expired_at, duration_minutes, peak_pilot_count, total_joins, rebalance_count, channel_change_count, video_systems, power_ceiling_mw, used_fixed_channels, city, region, country, latitude, longitude) VALUES ('SEED13', '2026-03-09T09:00:00', '2026-03-09T21:00:00', 720, 7, 8, 1, 2, '{\"dji_o3\":3,\"analog\":2,\"hdzero\":1,\"dji_o4\":1}', 200, 0, 'Orlando', 'FL', 'US', 28.5383, -81.3792);"
run "INSERT OR IGNORE INTO session_snapshots (session_code, created_at, expired_at, duration_minutes, peak_pilot_count, total_joins, rebalance_count, channel_change_count, video_systems, power_ceiling_mw, used_fixed_channels, city, region, country, latitude, longitude) VALUES ('SEED14', '2026-03-11T14:00:00', '2026-03-12T02:00:00', 720, 10, 12, 2, 4, '{\"dji_o3\":4,\"analog\":3,\"hdzero\":2,\"walksnail\":1}', 200, 1, 'Orlando', 'FL', 'US', 28.5383, -81.3792);"
run "INSERT OR IGNORE INTO session_snapshots (session_code, created_at, expired_at, duration_minutes, peak_pilot_count, total_joins, rebalance_count, channel_change_count, video_systems, power_ceiling_mw, used_fixed_channels, city, region, country, latitude, longitude) VALUES ('SEED15', '2026-03-14T10:00:00', '2026-03-14T22:00:00', 720, 5, 5, 0, 1, '{\"dji_o4\":2,\"analog\":2,\"hdzero\":1}', 0, 0, 'Orlando', 'FL', 'US', 28.5383, -81.3792);"

# Verify
COUNT=$(run "SELECT COUNT(*) FROM session_snapshots;")
echo "Done. $COUNT snapshots in database."
