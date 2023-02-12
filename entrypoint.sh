#!/bin/sh

HELP="
usage:

export API_HOST_ADDRESS=<hostname> or export API_HOST_ADDRESS=<ip>
export API_PORT=<port>

./entrypoint.sh [OPTIONS] [ARGUMENTS]

Ex. ./entrypoint.sh -i dev || ./entrypoint.sh -i prod || ./entrypoint.sh -h

optional arguments:
    -v, --version       show program's version number and exit
    -l, --local         run the program locally
    -i, --api           run the program on the API
        dev             run the program locally on the development environment
        prod            run the program on the production environment
    -h, --help          show this help message and exit
"

case $1 in
    "--local"|"-l")
        printf '%s' "\nStarting the application Local\n"
        /usr/local/bin/python certificate.py "${2}"
        ;;
    "--api"|"-i")
        if [ "${2}" = "dev" ]; then
            printf '%s' "\nStarting the api Development\n"
            /usr/local/bin/python api.py
        fi
        if [ "${2}" = "prod" ]; then
            printf '%s' "\nStarting the api Production\n"
            /usr/local/bin/gunicorn -w 4 -b "${API_HOST_ADDRESS}:${API_PORT}" -c api.py api:app
        fi
        ;;
    "--help"|"-h")
        printf '%s' "${HELP}"
        ;;
    *)
        printf '%s' "${HELP}"
        ;;
esac
