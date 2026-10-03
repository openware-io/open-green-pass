package redisquota

// acquireScript checks all four quota dimensions before mutating any of them.
// The caller passes units followed by global, team, target and owner limits.
// A non-positive limit means unlimited, matching quota.MemoryQuota.
const acquireScript = `local units = tonumber(ARGV[1])
if not units or units <= 0 then return -1 end
for i,key in ipairs(KEYS) do
  local current = tonumber(redis.call('GET', key) or '0')
  local limit = tonumber(ARGV[i + 1])
  if limit and limit > 0 and current + units > limit then return 0 end
end
for _,key in ipairs(KEYS) do redis.call('INCRBY', key, units) end
return 1`

// releaseScript decrements every dimension atomically and never leaves a
// negative counter. It is intentionally idempotent for a missing allocation.
const releaseScript = `local units = tonumber(ARGV[1])
if not units or units <= 0 then return -1 end
for _,key in ipairs(KEYS) do
  local current = tonumber(redis.call('GET', key) or '0')
  local next = current - units
  if next > 0 then redis.call('SET', key, next) else redis.call('DEL', key) end
end
return 1`
