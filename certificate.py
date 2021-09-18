"""
requires a recent enough python with idna support in socket
pyopenssl, cryptography and idna
"""
# -*- encoding: utf-8 -*-

import concurrent.futures
import sys
import json
import os
import logging

from collections import namedtuple
from time import sleep
from socket import socket, gaierror
from cryptography.x509.oid import NameOID, ExtensionOID
from cryptography import x509
from OpenSSL import SSL

import idna


HostInfo = namedtuple(
    field_names='cert hostname peername', typename='HostInfo')

HOSTS = [
    (sys.argv[1], sys.argv[2]),
]

OIDS = {
    "2.23.140.1.1": "Extended Validation (EV) Web Server SSL Digital Certificate",
    "2.16.840.1.114404.1.1.2.4.1": "Extended Validation (EV) Web Server SSL Digital Certificate",
    "2.23.140.1.2.1": "Domain Validation (DV) Web Server SSL Digital Certificate",
    "2.23.140.1.2.2": "Organization Validation (OV) Web Server SSL Digital Certificate",
    "2.23.140.1.2.3": "Organization Validation (OV) Web Server SSL Digital Certificate",
    "2.23.140.1.4.1": "Organization Validation (OV) Code Signing Certificate",
}


def verify_cert(cert):
    """
    verify notAfter/notBefore, CA trusted, servername/sni/hostname
    service_identity.pyopenssl.verify_hostname(client_ssl, hostname)
    issuer
    """
    return cert.has_expired()


def get_certificate(hostname, port):
    """ Get certificate from hostname:port """
    try:
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

    except ConnectionRefusedError:
        return sys.exit("\nThe host is not responding or don't exist\n")
    except gaierror:
        return sys.exit("\nThe hostname is not valid\n")


def get_crl(cert):
    """ Get CRL URLs from certificate """
    crl = []
    try:
        ext = cert.extensions.get_extension_for_oid(
            ExtensionOID.CRL_DISTRIBUTION_POINTS)
        crl_urls = ext.value
        for get_url in crl_urls:
            for url in get_url.full_name:
                crl.append(url.value)
        return crl
    except x509.ExtensionNotFound:
        return "CRL not found for this certificate!"


def get_alt_names(cert):
    """ Get alt names from certificate """
    try:
        ext = cert.extensions.get_extension_for_class(
            x509.SubjectAlternativeName)
        return ext.value.get_values_for_type(x509.DNSName)
    except x509.ExtensionNotFound:
        return None


def get_common_name(cert):
    """ Get common name from certificate """
    try:
        names = cert.subject.get_attributes_for_oid(NameOID.COMMON_NAME)
        return names[0].value
    except x509.ExtensionNotFound:
        return None


def get_issuer(cert):
    """ Get issuer from certificate """
    try:
        names = cert.issuer.get_attributes_for_oid(NameOID.COMMON_NAME)
        return names[0].value
    except x509.ExtensionNotFound:
        return None


def cert_type(cert):
    """ Get certificate type """
    try:
        ext = cert.extensions.get_extension_for_oid(
            ExtensionOID.CERTIFICATE_POLICIES)
        for oid in ext.value:
            oid_num = oid.policy_identifier.dotted_string

        for oid, desc in OIDS.items():
            if oid_num in oid:
                description = desc
        return description
    except UnboundLocalError:
        return "Type not found for this certificate!"


def days_left(cert):
    """ Get days left from certificate """
    days = cert.not_valid_after - cert.not_valid_before
    return days.days


def time_to_wait(waiting=86400):
    """ Sleep for time_to_wait seconds """
    sleep(waiting)
    return time_to_wait


def check_it_out(hostname, port):
    """ Check certificate """
    info_from_host = get_certificate(hostname, port)
    return print_basic_info(info_from_host)


def log_it_out(host_log_info):
    """ Log certificate """
    dir_log = os.path.dirname(os.path.abspath(__file__))
    log_path = f'{dir_log}/{os.path.basename(__file__).rsplit(".", 1)[0]}.log'
    logging.basicConfig(filename=f'{log_path}', level=logging.INFO,
                        format='%(levelname)s:%(asctime)s\n%(message)s',
                        datefmt='%d:%m:%y:%H:%M:%S', force=True)
    return logging.info(print_basic_info(host_log_info))


def print_basic_info(host_basic_info):
    """ Print basic info from certificate """
    try:
        out_info = {
            "commonName": f'{get_common_name(host_basic_info.cert)}',
            "subjectAltName": f'{get_alt_names(host_basic_info.cert)}',
            "issuer": f'{get_issuer(host_basic_info.cert)}',
            "type": f'{cert_type(host_basic_info.cert)}',
            "notBefore": f'{host_basic_info.cert.not_valid_before}',
            "notAfter": f'{host_basic_info.cert.not_valid_after}',
            "daysLeft": f'{days_left(host_basic_info.cert)}',
            "crl": f'{get_crl(host_basic_info.cert)}',
        }

        return json.dumps(out_info, indent=5)
    except AttributeError:
        return host_basic_info


if __name__ == '__main__':
    while True:
        with concurrent.futures.ThreadPoolExecutor(max_workers=4) as e:
            try:
                for hostinfo in e.map(lambda x: get_certificate(x[0], int(x[1])), HOSTS):
                    try:
                        if "--exit" in sys.argv[3]:
                            print(print_basic_info(hostinfo))
                            log_it_out(hostinfo)
                            sys.exit(0)
                        if sys.argv[3]:
                            time_to_wait(int(sys.argv[3]))
                            print(print_basic_info(hostinfo))
                            log_it_out(hostinfo)
                    except IndexError as e:
                        log_it_out(hostinfo)
                        print(print_basic_info(hostinfo))
                        time_to_wait()
            except KeyboardInterrupt:
                print('Terminated!')
                sys.exit(0)
