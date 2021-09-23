""" api """
# -*- encoding: utf-8 -*-

import json
import concurrent.futures
import flask


from flask.wrappers import Response
from settings import read_app_configs
from certificate import get_certificate, read_hosts, print_basic_info


app = flask.Flask(__name__)

for values in read_app_configs():
    # app.config['SERVER_NAME'] = f'{values[1]}:{values[2]}'
    app.config['ENV'] = values[3]
    app.config['DEBUG'] = values[4]


@app.route('/api/v1/cert/info', methods=['GET'])
def api_cert_info():
    """ Return a JSON with the certificate info """
    with concurrent.futures.ThreadPoolExecutor(max_workers=10) as executor:
        cert_list = []
        for hostinfo in executor.map(lambda x:
                                     get_certificate(x[0], int(x[1])),
                                     read_hosts()):
            cert_list.append(json.loads(print_basic_info(hostinfo)))

    return Response(json.dumps(cert_list, indent=4, default=str),
                    mimetype='application/json', status=200)


if __name__ == '__main__':
    app.run(host='0.0.0.0', port=5000, threaded=False, processes=1)
    # app.run()
