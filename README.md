# auto-withdraw
Simple auto-withdraw for EVM chains. It resends incoming transactions and replaces outcoming.

# Contact me
Jabber: aeternity@jabber.fr <br>
Tox: 6B524273004BE142DCD9A4B95473B01AA57C72D0CBEC13AC10641965A8C9BF53925AFFF228FE

# Setup
All you need is Golang and gcc compilator. To build just run `go build .` and you will get executable.<br>
Before running executable setup config. Replace null in endpoints to ["endpoint1", "endpoint2"].<br>
To load your accounts you need to put private keys to accounts.txt near executable.<br>
Endpoints should be WebSocket or IPC using geth client

# Issues
I'm newbie in development so its not suprise if there will be some issues. If you find one, please contact me or just open issue here.
