""" api """
# -*- encoding: utf-8 -*-

import json
import concurrent.futures
import flask


from flask.wrappers import Response
from settings import read_app_configs
from certificate import get_certificate, log_it_out, read_hosts, print_basic_info


app = flask.Flask(__name__)


@app.route('/api/v1/cert/info/all', methods=['GET'])
def api_cert_info():
    """ Return a JSON with the certificate info """
    with concurrent.futures.ThreadPoolExecutor(max_workers=10) as executor:
        cert_list = []
        for hostinfo in executor.map(lambda x:
                                     get_certificate(x[0], int(x[1])),
                                     read_hosts()):
            cert_list.append(json.loads(print_basic_info(hostinfo)))
            log_it_out(hostinfo)

    return Response(json.dumps(cert_list, indent=4, default=str),
                    mimetype='application/json', status=200)


@app.route('/api/v1/cert/info/<hostname>', methods=['GET'])
def api_cert_info_hostname(hostname):
    """ Return a JSON with the certificate info from hostname """
    with concurrent.futures.ThreadPoolExecutor(max_workers=10) as executor:
        cert_uniq = []
        for hostinfo in executor.map(lambda x:
                                     get_certificate(x[0], int(x[1])),
                                     read_hosts()):
            if hostinfo.hostname == hostname:
                cert_uniq.append(json.loads(print_basic_info(hostinfo)))
                log_it_out(hostinfo)

    return Response(json.dumps(cert_uniq, indent=4, default=str),
                    mimetype='application/json', status=200)


@app.route('/api/v1/cert/info/commonName', methods=['GET'])
def api_cert_info_common_name():
    """ Return a JSON with the certificate commonName info """
    with concurrent.futures.ThreadPoolExecutor(max_workers=10) as executor:
        cert_list_common_name = []
        for hostinfo in executor.map(lambda x: get_certificate(x[0], int(x[1])),
                                     read_hosts()):
            cert_list_common_name.append(json.loads(
                print_basic_info(hostinfo))['commonName'])
            log_it_out(hostinfo)

    return Response(json.dumps(cert_list_common_name, indent=4, default=str),
                    mimetype='application/json', status=200)


@app.route('/api/v1/cert/info/subjectAltName', methods=['GET'])
def api_cert_info_subject_alt_name():
    """ Return a JSON with the certificate subjectAltName info """
    with concurrent.futures.ThreadPoolExecutor(max_workers=10) as executor:
        cert_list_subject_alt_name = []
        for hostinfo in executor.map(lambda x: get_certificate(x[0], int(x[1])),
                                     read_hosts()):
            cert_list_subject_alt_name.append(json.loads(
                print_basic_info(hostinfo))['subjectAltName'])
            log_it_out(hostinfo)

    return Response(json.dumps(cert_list_subject_alt_name, indent=4, default=str),
                    mimetype='application/json', status=200)


if __name__ == '__main__':

    for values in read_app_configs():
        app.config['HOST'] = values[1]
        app.config['PORT'] = values[2]
        app.config['ENV'] = values[3]
        app.config['DEBUG'] = values[4]

        app.run(host=app.config['HOST'], port=app.config['PORT'],
                threaded=False, processes=1, debug=app.config['DEBUG'])
