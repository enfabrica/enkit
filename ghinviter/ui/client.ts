import {createChannel, createClient} from 'nice-grpc-web';
import {
  AccountClient,
  AccountDefinition,
// } from '../proto/ghinviter/proto/user';
} from '../proto/user';

// bazel-bin/ghinviter/proto/user-ts/ghinviter/proto/user.js
// bazel-bin/ghinviter/ui/main.js

const channel = createChannel("");
const client: AccountClient = createClient(AccountDefinition, channel);

export { client };

console.log("got", createChannel);
