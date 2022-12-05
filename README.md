invoicepayer
============

the simplest lightning faucet
-----------------------------

Compile:

```
make
```

Run:

```
CLN=/home/yourself/.lightning/signet/lightning-rpc ./invoicepayer
```

This will start a server at http://127.0.0.1:5556 that will serve an HTML page where people can paste lightning invoices and it will pay them with the given `lightningd` instance.

Once the user clicks on "pay invoice" the UI will change to a very barebones-but-functional live-tracking page of the status of the payment.

Be sure to only run this on a trusted environment or on testnet/signet/regtest etc so you don't lose money.
