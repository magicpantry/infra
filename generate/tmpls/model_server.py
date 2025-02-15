import argparse
import asyncio
import concurrent.futures
import io
import json
import sys
import traceback

from PIL import Image
import numpy as np
import tensorflow as tf
import grpc
import grpc_reflection.v1alpha.reflection

import gen.infra.proto.model_pb2 as modelpb
import gen.infra.proto.model_pb2_grpc as modelpb_grpc


class _Servicer(modelpb_grpc.ModelServiceServicer):

    def __init__(self, models):
        self._models = models

    def Execute(self, request, context):
        response = modelpb.ExecuteResponse()
        try:
            for value in request.requests:
                if value.model_id not in self._models:
                    continue
                model = self._models[value.model_id]
                model_response = modelpb.ModelResponse()
                model_response.model_id = value.model_id

                if value.WhichOneof('requests') == 'tag_text_requests':
                    predictions = model.predict([v.text for v in value.tag_text_requests.requests])
                    for prediction in predictions:
                        sub = modelpb.TagResponse()
                        sub.confidences.extend(prediction)
                        model_response.tag_responses.append(sub)
                if value.WhichOneof('requests') == 'tag_image_requests':
                    predictions = model.predict(
                            np.stack([tf.keras.utils.img_to_array(Image.open(io.BytesIO(v.raw)))
                                for v in value.tag_image_requests.requests]))
                    for prediction in predictions:
                        sub = modelpb.TagResponse()
                        sub.confidences.extend(prediction)
                        model_response.tag_responses.append(sub)

                response.responses.append(model_response)
            return response
        except Exception as exc:
            traceback.print_exception(exc)
            raise exc


async def _load_model(name, path):
    return name, tf.keras.models.load_model(path)


async def _serve(host, config):
    server = grpc.server(concurrent.futures.ThreadPoolExecutor(max_workers=8))

    tasks = []
    for model_config in config['models']:
        tasks.append(_load_model(model_config['name'], model_config['path']))
    results = await asyncio.gather(*tasks)

    models = dict()
    for result in results:
        models[result[0]] = result[1]

    service_names = (
        modelpb.DESCRIPTOR.services_by_name['ModelService'].full_name,
        grpc_reflection.v1alpha.reflection.SERVICE_NAME,
    )
    modelpb_grpc.add_ModelServiceServicer_to_server(_Servicer(models), server)
    grpc_reflection.v1alpha.reflection.enable_server_reflection(service_names, server)

    server.add_insecure_port(host)
    server.start()

    print(f'listening at {host}')

    server.wait_for_termination()


if __name__ == '__main__':
    tf.keras.utils.disable_interactive_logging()

    parser = argparse.ArgumentParser()
    parser.add_argument('--port')
    parser.add_argument('--config-path')
    args = parser.parse_args()

    with open(args.config_path, 'r') as fp:
        config = json.loads(fp.read())

    host = f'[::]:{args.port}'

    print(f'listening at: {args.port}', file=sys.stderr)

    asyncio.run(_serve(host, config))
