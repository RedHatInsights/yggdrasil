# `yggd`

`yggd` is the main system daemon that listens for remote commands and routes
them to their destined workers. It consists of three major data structures:
`transport.Transporter`, `main.Client` and `work.Dispatcher`. These three data
structures interact concurrently to allow a continuous, uninterrupted
bidirection flow of data through the program.

### `transport.Transporter`
`transport.Transporter` is an interface that provides a pair of "send" and
"receive" functions to send and receive data through an underlying network
transport. There are two concrete data structures that implement the Transporter
interface: MQTT and HTTP. These two data structures provide identical APIs by
way of implementing the `transporter.Transport` interface. Each is backed by a
native network protocol, but abstract the implementation details from callers of
the `transport.Transporter` interface. `transport.Transporter` receives data
asynchronously. When data is received, it asynchronously calls a function
handler that was provided when the transport was set up. Sending data using a
`transport.Transporter` is done by calling its `Tx` method.

### `work.Dispatcher`

`work.Dispatcher` is a data structure that implements a D-Bus interface and
maintains a list of knows "workers". It sends and receives data through two
channels: `Inbound` and `Outbound`. When a `work.Dispatcher` receives data on
its Inbound channel, it attempts to find a worker the data is destined for,
serialize the data into a D-Bus message, and sends the data through D-Bus to the
target worker. When a worker calls the
`com.redhat.Yggdrasil1.Dispatcher1.Transmit` D-Bus method, the `work.Dispatcher`
receives the data; in its implementation of `Transmit` the data is deserialized
and sent to the `Outbound` channel. Along with the sending data, second channel
is sent to this `Outbound` channel. The `work.Dispatcher` will asynchronously
wait for a value to return on this second channel. If no data is received, this
routine will eventual time out and close the response channel.

### `main.Client`

The high-level `main.Client` can be thought of as the orchestrator between the
two. When a `transport.Transporter` is created, a method on `Client` is passed
to the `transport.Transporter` as its `RxHandlerFunc`. This client method is
then invoked when data is received by the `transport.Transporter`. That method
takes the data received by the `transport.Transporter` and sends it to the
`work.Dispatcher`'s `Inbound` channel. Conversely, the `main.Client` establishes
a long running goroutine that receives values from the `dispatcher.Dispatcher`'s
`Outbound` channel. When a value is received, it calls the
`transport.Transporter`'s `Tx` function.

Another way to visualize it; the Rx/Tx functions of `transport.Transporter`
function as a flowing loop of data between the `transport.Transporter` and the
`main.Client`. The Inbound/Outbound channels of `work.Dispatcher` function as
another flowing loop of data between the `main.Client` and the
`work.Dispatcher`.

                  ┌───Rx───┐            ┌──Inb───┐              
                  │        │            │        │              
    ┌─────────────┼─┐    ┌─┼────────────┼─┐    ┌─┼─────────────┐
    │             │ │    │ └──────►─────┘ │    │ │             │
    │   Transport ▲ │    │     Client     │    │ ▼ Dispatcher  │
    │             │ │    │ ┌─────◄──────┐ │    │ │             │
    └─────────────┼─┘    └─┼────────────┼─┘    └─┼─────────────┘
                  │        │            │        │              
                  └───Tx───┘            └──Outb──┘              

`main.Client` also implements the `com.redhat.Yggdrasil1` D-Bus interface. This
interface is less critical to the dispatcher/worker interaction, but does
provide a general-purpose API for clients (such as `yggctl`) to interact with
`yggd` and its workers.
