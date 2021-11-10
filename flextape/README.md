# Flextape

Flextape is our out-of-band license management server that gates individual
actions run via bazel until the required license is available from the
vendor-provided flexlm license server.

This is necessary since the vendor-provided solutions do not support queueing,
causing actions to fail if there is too much concurrency with other actions that
require the same license type.

The `flextape` server maintains a count of licenses out-of-band with the
vendor-provided license manager, and individual actions are wrapped with a
client that obtains a token from the `flextape` server before running their
command. The `flextape` protocol has keepalive mechanisms that will aggressively
expire the token if clients become unresponsive, unblocking subsequent actions.

More details in [this
doc](https://docs.google.com/document/d/1TNqbBprpcNU9tTHVCFzRwaQoHlGFdjkw221C5p9UsAw/edit).