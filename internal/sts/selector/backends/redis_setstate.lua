-- Copyright 2026 Thomson Reuters
--
-- Licensed under the Apache License, Version 2.0 (the "License");
-- you may not use this file except in compliance with the License.
-- You may obtain a copy of the License at
--
--     http://www.apache.org/licenses/LICENSE-2.0
--
-- Unless required by applicable law or agreed to in writing, software
-- distributed under the License is distributed on an "AS IS" BASIS,
-- WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
-- See the License for the specific language governing permissions and
-- limitations under the License.

-- Atomic compare-and-swap for rate limit state.
-- A write succeeds when: (1) no existing state, (2) new rate limit window
-- (newer reset_at), or (3) same window with lower remaining.
--
-- KEYS[1]: rate limit key
-- ARGV[1]: new reset_at (unix timestamp)
-- ARGV[2]: new remaining
-- ARGV[3]: new last_updated (unix timestamp)
-- ARGV[4]: TTL in seconds
--
-- Returns 1 if written, 0 if stale (silently discarded).
local cur = redis.call('HMGET', KEYS[1], 'reset_at', 'remaining')
local cur_reset = tonumber(cur[1])
local new_reset = tonumber(ARGV[1])
local new_rem = tonumber(ARGV[2])

if not cur_reset or (new_reset > cur_reset) or (new_reset == cur_reset and new_rem < tonumber(cur[2])) then
    redis.call('HMSET', KEYS[1], 'reset_at', ARGV[1], 'remaining', ARGV[2], 'last_updated', ARGV[3])
    redis.call('EXPIRE', KEYS[1], tonumber(ARGV[4]))
    return 1
end
return 0
