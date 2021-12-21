import React from 'react';
import logo from './logo.svg';
import './App.css';
import {Button} from '@material-ui/core';
import {EchoControllerClient} from 'rpc/rpc/test/example/rpc/echo_grpc_web_pb';
import {EchoRequest, EchoResponse} from 'rpc/rpc/test/example/rpc/echo_pb';


function App() {
    const client = new EchoControllerClient("http://localhost:8080");
    let request = new EchoRequest();
    request.setMessage('Hello World!');
    let stream = client.echo(request, {});
    stream.on('data', function(response: EchoResponse) {
        console.log(response.getMessage());
    });
    stream.on('status', function(status) {
        console.log(status.code);
        console.log(status.details);
        console.log(status.metadata);
    });
    stream.on('end', function() {
        // stream end signal
    });

    return (
        <div className="App">
            <header className="App-header">
                <img src={logo} className="App-logo" alt="logo"/>
                <p>
                    Edit <code>src/App.tsx</code> and save to reload.
                </p>
                <a
                    className="App-link"
                    href="https://reactjs.org"
                    target="_blank"
                    rel="noopener noreferrer"
                >
                    Hello Worlds!
                </a>
            </header>
            <Button onClick={() => {}}  color="primary">Hello Material</Button>;
        </div>
    );
}

export default App;
