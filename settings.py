import yaml


def read_hosts():
    with open('settings.yml', 'r') as file:
        list_hosts = []
        host_info = yaml.load(file, Loader=yaml.Loader)
        hosts = host_info['hosts']

        for values in hosts:
            TMP = list_hosts.append([values['url'], values['port']])

        HOSTS = list_hosts

    return HOSTS


def read_check_time():
    with open('settings.yml', 'r') as file:
        settings = yaml.load(file, Loader=yaml.Loader)
        check_time = settings['check_time']

    return check_time

# print(read_hosts())
# print(read_check_time())
