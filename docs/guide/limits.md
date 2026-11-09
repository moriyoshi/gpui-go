# Limits

Limits are enforced, and exceeding one is an explicit error rather than silent loss.

| Limit | Value |
| --- | --- |
| Protobuf message | 4 MiB |
| Elements per view | 10,000 |
| Tree depth | 32 |
| Queued foreground messages | 256 |
| Unacknowledged events | 1024 |
| Single resource upload | 2 MiB |
| Resources per session | 128, and 32 MiB of source bytes in total |
| Decoded raster image | 4096 by 4096 pixels |
| Models per session | 1024, at 64 KiB per value and 128 observers per model |

Virtual lists still transmit every row within these limits: virtualization covers native layout and painting, not paging. Animated image formats currently show their first frame.
