# Quickstart: Resolve User IDs and Channel IDs in Output

**Feature**: 003-resolve-ids-in-output

---

## What Changes

After this feature, `history`, `messages`, and `search` commands display human-readable
names instead of raw Slack IDs in all output formats.

**Before:**
```
$ slackseek history general --format table
┌──────────────────────┬───────────┬──────────────────────────┐
│ Timestamp            │ User      │ Text                     │
├──────────────────────┼───────────┼──────────────────────────┤
│ 2026-01-01T10:00:00Z │ U01234567 │ Hello world              │
│ 2026-01-01T10:01:00Z │ U09876543 │ How's it going?          │
└──────────────────────┴───────────┴──────────────────────────┘
```

**After:**
```
$ slackseek history general --format table
┌──────────────────────┬───────┬──────────────────────────┐
│ Timestamp            │ User  │ Text                     │
├──────────────────────┼───────┼──────────────────────────┤
│ 2026-01-01T10:00:00Z │ alice │ Hello world              │
│ 2026-01-01T10:01:00Z │ bob   │ How's it going?          │
└──────────────────────┴───────┴──────────────────────────┘
```

---

## How It Works

1. Each command fetches channels and users from the **existing cache** (feature 002).
   For a warm cache this is a disk read — no additional API calls.
2. A `slack.Resolver` is built from those lists: two hash maps keyed on ID.
3. The output layer substitutes IDs with names when rendering text/table.
4. JSON output adds `user_display_name` and populates `channel_name` for all commands.

---

## Fallback Behaviour

| Scenario | Output |
|----------|--------|
| Cache warm, user found | Display name (or real name if display name empty) |
| Cache warm, user not found | Raw `U…` ID |
| `--no-cache` flag | Raw `U…` ID (no extra API call) |
| Cache warm, channel found | Channel name |
| Cache warm, channel not found | Raw `C…` ID |

---

## Developer Notes

### Adding the Resolver in a new cmd/ command

If you write a new command that outputs messages, construct and pass the resolver:

```go
users, err := client.ListUsers(ctx)
if err != nil {
    fmt.Fprintln(os.Stderr, "Warning: could not load users for name resolution:", err)
}
channels, err := client.ListChannels(ctx, nil, false)
if err != nil {
    fmt.Fprintln(os.Stderr, "Warning: could not load channels for name resolution:", err)
}
resolver := slack.NewResolver(users, channels) // nil slices are safe
return output.PrintMessages(cmd.OutOrStdout(), output.Format(flagFormat), messages, resolver)
```

### Unit testing the Resolver

```go
users := []slack.User{
    {ID: "U001", DisplayName: "alice"},
    {ID: "U002", RealName: "Bob Smith"},   // empty DisplayName
}
channels := []slack.Channel{
    {ID: "C001", Name: "general"},
}
r := slack.NewResolver(users, channels)
assert.Equal(t, "alice", r.UserDisplayName("U001"))
assert.Equal(t, "Bob Smith", r.UserDisplayName("U002"))
assert.Equal(t, "UUNKNOWN", r.UserDisplayName("UUNKNOWN"))  // fallback
assert.Equal(t, "general", r.ChannelName("C001"))
assert.Equal(t, "C999", r.ChannelName("C999"))              // fallback
```
