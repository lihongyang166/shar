import google.protobuf.pyext
import google.protobuf.util
import pypb.models_pb2
from nats.aio.client import Client as NATS
from nats.aio.msg import Msg
from nats.aio.errors import ErrConnectionClosed, ErrTimeout, ErrNoServers
import asyncio
import json
import msgpack
from google.protobuf.any_pb2 import Any

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

        # Construct the workflow launch protobuf request:
        print('setting up PB')

        b = msgpack.packb(vars)

        req = pypb.models_pb2.LaunchWorkflowRequest(processId=id,vars=b)

        print('serialising PB')

        payload = req.SerializeToString()

        print('setting headers')

        # Define headers:
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
       
        # Publish the message:
        try:
            await nc.request(msg.subject,msg.data,30,headers=headers)
        except ErrConnectionClosed as e:
            print(f"Could not send message: {e}")
        await nc.close()

if __name__ == "__main__":
    wfpid = "SendEmail_test"
    version = "v1.1.1212"

    shar = client("nats://localhost:4222",version)

    vars = {"To":"matthew.brazel@vitrifi.net", "Subject": "PYTEST!", "Body": "PYTEST!"}
    loop = asyncio.get_event_loop()
    loop.run_until_complete(shar.launch_workflow(id=wfpid,vars=vars))
    loop.close()