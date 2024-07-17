import google.protobuf.pyext
import google.protobuf.util
import pypb.models_pb2
from nats.aio.client import Client as NATS
from nats.aio.msg import Msg
from nats.aio.errors import ErrConnectionClosed, ErrTimeout, ErrNoServers
import nats.js
import asyncio
import base64
import json
from google.protobuf.any_pb2 import Any
import pygob
# def dict_to_bytes(data_dict):
#     # Serialize the dictionary to JSON and then encode to bytes
#     b64_str = base64.urlsafe_b64encode(json.dumps(data_dict).encode())
#     return b64_str
def base128_encode(data):
    result = []
    num = 0
    bits = 0

    for byte in data:
        num = (num << 8) | byte
        bits += 8

        while bits >= 7:
            bits -= 7
            result.append((num >> bits) & 0x7F)

    # If there are remaining bits, pad with zeros
    if bits > 0:
        result.append((num << (7 - bits)) & 0x7F)

    return bytes(result)

def dict_to_bytes(d):
    # Convert dictionary to byte representation
    items = []
    for key, value in d.items():
        items.append(key.encode('utf-8'))
        items.append(value.encode('utf-8'))
    
    return b','.join(items)

def encode_dict_to_base128(d):
    # Convert dictionary to bytes
    data_bytes = dict_to_bytes(d)
    
    # Encode bytes using base128
    encoded_data = base128_encode(data_bytes)
    
    return encoded_data

class client():
    def __init__(self,nats_url,version):
        self.nats_url = nats_url
        self.version = version
        print('nats server address: ' + self.nats_url)

    async def launch_workflow(self,id,vars):     
        # Connect to NATS!
        nc = NATS()
        try:
            await nc.connect(servers=[self.nats_url])
        except ErrNoServers as e:
            print(f"Could not connect to any NATS servers: {e}")
            return
         # Initialize JetStream context
        # print('setting up JS')
        # js = nc.jetstream()
        # data = dict_to_bytes(vars)
        # Construct the workflow launch protobuf request:
        print('setting up PB')
        # vars = 'test'

        n = {}
        b = b''
        for key, value in vars.items():
            k = pygob.dump(key)
            v = pygob.dump(value)
            b += k
            b += v
        print(b)
        # encoded = pygob.dump(vars)
        # b = base64.b64encode(vars)
        # encoded_data = base128_encode(b'To:matthew.brazel@vitrifi.net, Subject: PYTEST!, Body: PYTEST!')
        # encoded_data = encode_dict_to_base128(vars)
        # print(encoded)
        req = pypb.models_pb2.LaunchWorkflowRequest(processId=id,vars=b)

        # Populate vars field with dictionary data
        print('setting up dict')

        # for key, value in vars.items():
        #     any_value = Any()
        #     any_value.Pack(value)
        #     req.vars[key].CopyFrom(any_value)

        print('serialising PB')
        # Pack protobuf request data:
        payload = req.SerializeToString()
        print('setting headers')
        # Define headers
        headers = {
            "Shar-Compat": "1.1.503",
            "Shar-Namespace": "default",
            "Shar-Version": self.version,            
        }
        print('setting nats msg')
        msg = Msg(
            nc,
            subject="WORKFLOW.Api.LaunchProcess",
            data=payload,
            headers=headers
        )
        print('sending nats msg')
        # Publish a message to 'foo'
        try:
            await nc.request(msg.subject,msg.data,30,headers=headers)
            # await js.publish(msg.subject,msg.data,30,msg.subject,msg.headers)
        except ErrConnectionClosed as e:
            print(f"Could not send message: {e}")
        await nc.close()

if __name__ == "__main__":
    wfpid = "SendEmail_test"
    version = "v1.1.1212"

    shar = client("nats://localhost:4222",version)

    vars = {"To":"matthew.brazel@vitrifi,net", "Subject": "PYTEST!", "Body": "PYTEST!"}
    # vars = b'"To":"matthew.brazel@vitrifi,net", "Subject": "PYTEST!", "Body": "PYTEST!"'
    loop = asyncio.get_event_loop()
    loop.run_until_complete(shar.launch_workflow(id=wfpid,vars=vars))
    loop.close()