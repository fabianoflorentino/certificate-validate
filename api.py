""" api """
import flask
import sys
import os
import concurrent.futures

from certificate import get_certificate, print_basic_info
from settings import read_hosts


app = flask.Flask(__name__)
app.config['DEBUG'] = True


@app.route('/api/v1/cert/info', methods=['GET'])
def cert_info():
    """ get certificate info """
    with concurrent.futures.ThreadPoolExecutor(max_workers=4) as e:
        for hostinfo in e.map(lambda x: get_certificate(x[0], int(x[1])), read_hosts()):
            cert_infos = print_basic_info(hostinfo)

    return print(cert_infos)


if __name__ == '__main__':
    app.run(host='0.0.0.0', port='5000')
