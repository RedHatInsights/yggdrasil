# yggdrasil "http" worker

This "http" worker is a simple yggdrasil worker that demonstrates the explicit
use of the HTTP request functionality provided by
`com.redhat.Yggdrasil1.Dispatcher1`.

## Running

The worker can run directly without needing to install it first (`go run .`). By
default, the worker will attempt to connect to an appropriate D-Bus (session or
system), depending on whether `DBUS_SESSION_BUS_ADDRESS` is defined.

## Usage

The worker expects to receive messages in JSON conforming to the following JSON
schema definition.

### Schema
```json
{
    "type": "object",
    "properties": {
        "method": { "type": "string" },
        "url": { "type": "string" },
        "headers": {
            "type": "object",
            "patternProperties": {
                "^.*$": { "type": "string" }
            }
        },
        "body": { "type": "string" }
    },
    "required": ["method", "url" ]
}
```

### Data

```json
{
    "method": "GET",
    "url": "http://httpbin.org/get",
    "headers": {
        "User-Agent": "yggdrasil-http-worker"
    }
}
```

It will unmarshal received message data and invoke the
`com.redhat.Yggdrasil1.Dispatcher1.Request` D-Bus method, using the supplied
fields. The response code, headers and body are logged.

### Examples

This command will send a message instructing the echo worker to send an HTTP GET
request to the URL "http://httpbin.org/get". 

```
echo '{"method": "GET", "url": "http://httpbin.org/get", "headers": {"User-Agent":"yggdrasil-http-worker"}}' | \
    go run ./cmd/yggctl generate data-message --directive http - \
    pub -broker tcp://localhost:1883 -topic yggdrasil/$(hostname)/data/in
```
