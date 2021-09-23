#!/bin/sh
cd $PWD

case "$1" in
    "--local")
        /bin/echo -e "\nStarting the application\n"
        /usr/local/bin/python certificate.py $2
        ;;
    "--api")
        /bin/echo -e "\nStarting the api\n"
        /usr/local/bin/python api.py
        ;;
    "--help")
        /bin/echo -e "\nUsage: $0 [--local|--api]\n"
        ;;
    *)
        /bin/echo -e "\nUsage: $0 [--local|--api]\n"
        exit 0
        ;;
esac