### :tada: This is the enkit relay! :tada:
* What does it do?
  * It lets people join any tcp client and any tcp server from anywhere
  * It accomplishes this by translating per-connection tcp <=> websocket <=> tcp
* What's needed to use it?
  * Any server (e.g. sharing peer), must designate a password at the start of their sharing session
  * The relay server will give them a non-guessable session id associated with that password
  * In order for a client (e.g. consuming peer) to connect, they must provide the client url and password.
* Show me a diagram of how it works!
  ![](assets/tcprelay.png?raw=true)
* How do I use it?
  * With GRPC
  
    ``todo``
  * With a TCP client
    
    ``todo``

  * Peer to Peer
  
    ``todo``
* What currently uses it
  * ``todo link to enfuse``
* FAQ
  * Why is the relay server over wss instead of tcp again?
    * By using websockets, we can use h2c or any tcp based transport. We are not limited by 
      browser compatibility.
    * We can theoretically use it from a browser. Virtual terminals and the like.
    * WSS read/writes the full buffer every time, so it's easy when reconstructing/deconstructing data.