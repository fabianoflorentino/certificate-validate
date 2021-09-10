# -*- encoding: utf-8 -*-
# requires a recent enough python with idna support in socket
# pyopenssl, cryptography and idna
import idna
import concurrent.futures
import sys
import json
import logging

from OpenSSL import SSL
from cryptography import x509
from cryptography.x509.oid import NameOID
from socket import socket
from collections import namedtuple
from time import sleep, time

HostInfo = namedtuple(
    field_names='cert hostname peername', typename='HostInfo')

HOSTS = [
    (sys.argv[1], sys.argv[2]),
]


def verify_cert(cert, hostname):
    # verify notAfter/notBefore, CA trusted, servername/sni/hostname
    return cert.has_expired()
    # service_identity.pyopenssl.verify_hostname(client_ssl, hostname)
    # issuer


def get_certificate(hostname, port):
    hostname_idna = idna.encode(hostname)
    sock = socket()

    sock.connect((hostname, port))
    peername = sock.getpeername()
    ctx = SSL.Context(SSL.SSLv23_METHOD)  # most compatible
    ctx.check_hostname = False
    ctx.verify_mode = SSL.VERIFY_NONE

    sock_ssl = SSL.Connection(ctx, sock)
    sock_ssl.set_connect_state()
    sock_ssl.set_tlsext_host_name(hostname_idna)
    sock_ssl.do_handshake()
    cert = sock_ssl.get_peer_certificate()
    crypto_cert = cert.to_cryptography()
    sock_ssl.close()
    sock.close()

    return HostInfo(cert=crypto_cert, peername=peername, hostname=hostname)


def get_alt_names(cert):
    try:
        ext = cert.extensions.get_extension_for_class(
            x509.SubjectAlternativeName)
        return ext.value.get_values_for_type(x509.DNSName)
    except x509.ExtensionNotFound:
        return None


def get_common_name(cert):
    try:
        names = cert.subject.get_attributes_for_oid(NameOID.COMMON_NAME)
        return names[0].value
    except x509.ExtensionNotFound:
        return None


def get_issuer(cert):
    try:
        names = cert.issuer.get_attributes_for_oid(NameOID.COMMON_NAME)
        return names[0].value
    except x509.ExtensionNotFound:
        return None


def cert_type(cert):
    for ext in cert.extensions:
        if ext.oid.dotted_string == '2.23.140.1.2.1':
            return 'DV type'
        if ext.oid.dotted_string == '2.23.140.1.2.2':
            return 'OV type'
        if ext.oid.dotted_string == '2.23.140.1.1':
            return 'EV type'
        return 'Normal certificate type'


def time_to_wait(time_to_wait=86400):
    sleep(time_to_wait)
    return time_to_wait


def print_basic_info(hostinfo):
    s = {
        "commonName": f'{get_common_name(hostinfo.cert)}',
        "SAN": f'{get_alt_names(hostinfo.cert)}',
        "issuer": f'{get_issuer(hostinfo.cert)}',
        "notBefore": f'{hostinfo.cert.not_valid_before}',
        "notAfter": f'{hostinfo.cert.not_valid_after}',
        "type": f'{cert_type(hostinfo.cert)}',
    }
    # print(json.dumps(s, indent=5))
    return json.dumps(s, indent=5)


def check_it_out(hostname, port):
    hostinfo = get_certificate(hostname, port)
    return print_basic_info(hostinfo)


def log_it_out(hostinfo):
    log_path = '/app/certificate.log'
    logging.basicConfig(filename=f'{log_path}', level=logging.INFO,
                        format='%(levelname)s:%(asctime)s\n%(message)s',
                        datefmt='%d:%m:%y:%H:%M:%S')
    return logging.info(print_basic_info(hostinfo))


if __name__ == '__main__':
    while True:
        with concurrent.futures.ThreadPoolExecutor(max_workers=4) as e:
            try:
                for hostinfo in e.map(lambda x: get_certificate(x[0], int(x[1])), HOSTS):
                    try:
                        if sys.argv[3]:
                            time_to_wait(int(sys.argv[3]))
                            print(print_basic_info(hostinfo))
                            log_it_out(hostinfo)
                    except IndexError as e:
                        log_it_out(hostinfo)
                        print(print_basic_info(hostinfo))
                        time_to_wait()
            except KeyboardInterrupt:
                print(f'Terminated!')
                quit()
