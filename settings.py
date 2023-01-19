""" settings """
# -*- encoding: utf-8 -*-

import yaml


def read_hosts():
    """ Reads the hosts from the settings.yml file """
    with open('config/settings.yml', 'r', encoding='utf-8') as file:
        list_hosts = []
        host_info = yaml.load(file, Loader=yaml.Loader)
        hosts = host_info['hosts']

        for values in hosts:
            list_hosts.append([values['url'], values['port']])

        hosts = list_hosts

    return hosts


def read_check_time():
    """ Reads the check time from the settings.yml file """
    with open('config/settings.yml', 'r', encoding='utf-8') as file:
        settings = yaml.load(file, Loader=yaml.Loader)
        check_time = settings['check_time']

    return check_time


def read_app_configs():
    """ Reads the app configs from the settings.yml file """
    with open('config/settings.yml', 'r', encoding='utf-8') as file:
        list_app_configs = []
        settings = yaml.load(file, Loader=yaml.Loader)
        app_configs = settings['app_configs']

        for values in app_configs:
            list_app_configs.append([
                values['name'],
                values['host'],
                values['port'],
                values['environment'],
                values['debug'],
            ])

    return list_app_configs
