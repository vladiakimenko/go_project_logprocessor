#!/usr/bin/env bash

# cli args
FILE_PATH="$1"
MODE="$2"
MODE_SPECIFIC_ARG="$3"

if [[ -z "$FILE_PATH" || -z "$MODE" || -z "$MODE_SPECIFIC_ARG" ]]; then
  echo "Usage: $0 <file_path> <mode: total> [rows count]"
  exit 1
fi

if [[ "$MODE" != "total" ]]; then
    echo "Unknown mode: $MODE. Only 'total' is supported now"
    exit 1
fi

# progress bar
show_progress() {
    local current="$1"
    local total="$2"
    local percentage=$((current * 100 / total))
    printf "\r%d%% (%d/%d)" $percentage $current $total
}


# sample data
declare -A ENDPOINTS_METHODS
ENDPOINTS_METHODS["/api/health"]="GET"
ENDPOINTS_METHODS["/api/auth"]="POST"
ENDPOINTS_METHODS["/api/users"]="GET POST"
ENDPOINTS_METHODS["/api/products"]="GET POST"
ENDPOINTS_METHODS["/api/orders"]="GET POST"
ENDPOINTS_METHODS["/api/reports"]="GET POST"
ENTITY_ENDPOINTS=("/api/users" "/api/products" "/api/orders" "/api/reports")
ENTITY_METHODS=(GET PATCH DELETE)
LIST_STATUS=(200 401 500 522)
POST_STATUS=(201 400 401 500 522)
ENTITY_GET_STATUS=(200 401 403 404 500 522)
ENTITY_DELETE_STATUS=(204 401 403 404 500 522)
ENTITY_PATCH_STATUS=(200 400 401 403 404 500 522)

# randomizers
random_ip() {
    local ips=(
        "127.0.0.1"
        "10.0.0.50"
        "10.0.0.51"
        "172.17.0.100"
        "172.17.0.101"
        "203.0.113.45"
        "198.51.100.23"
        "192.0.2.100"
        "54.123.45.67"
        "35.201.123.45"
        "13.107.21.200"
        "104.16.123.45"
        "151.101.120.133"
        "23.235.47.133"
    )
    
    local ips_count=${#ips[@]}
    local random_index=$((RANDOM % ips_count))
    echo "${ips[$random_index]}"
}

random_uuid() {
    uuidgen
}

random_request() {
    local existing_keys allowed_methods keys_count methods_count random_index
    local endpoint method url

    # random endpoint
    existing_keys=("${!ENDPOINTS_METHODS[@]}")
    keys_count=${#existing_keys[@]}
    random_index=$((RANDOM % keys_count))
    endpoint="${existing_keys[$random_index]}"

    # random method
    allowed_methods=(${ENDPOINTS_METHODS[$endpoint]})
    if [[ " ${ENTITY_ENDPOINTS[*]} " == *" $endpoint "* ]]; then
        allowed_methods+=("${ENTITY_METHODS[@]}")
    fi
    methods_count=${#allowed_methods[@]}
    random_index=$((RANDOM % methods_count))
    method="${allowed_methods[$random_index]}"

    # if method in GET, PATCH, DELETE and endpoint is one of the ENTITY_ENDPOINTS - add random uuid to the url
    if [[ "$method" =~ ^(PATCH|DELETE|GET)$ ]] && [[ " ${ENTITY_ENDPOINTS[*]} " == *" $endpoint "* ]]; then
        if [[ "$method" == "GET" ]]; then
            if (( RANDOM % 2 == 0 )); then
                url="$endpoint"                 # list endpoint
            else
                url="$endpoint/$(random_uuid)"  # detail endpoint
            fi
        else    # PATCH, DELETE
            url="$endpoint/$(random_uuid)"
        fi
    else
        url="$endpoint"
    fi
    
    echo "$method,$url"
}

random_status() {
    local method="$1" url="$2"
    local parts_count allowed_statuses
    local status

    # is this a detail request? (url has 3 parts)
    IFS='/' read -ra parts <<< "$url"
    parts_count=${#parts[@]}
    if (( parts_count == 4 )); then
        case "$method" in
            GET) status_list=("${ENTITY_GET_STATUS[@]}") ;;
            PATCH) status_list=("${ENTITY_PATCH_STATUS[@]}") ;;
            DELETE) status_list=("${ENTITY_DELETE_STATUS[@]}") ;;
        esac
    else
        case "$method" in
            GET) status_list=("${LIST_STATUS[@]}") ;;
            POST) status_list=("${POST_STATUS[@]}") ;;
        esac
    fi

    if (( RANDOM % 10 < 9 )); then  # 90% chance for successful request
        status="${status_list[0]}"
    else
        status="${status_list[RANDOM % ${#status_list[@]}]}"
    fi

    echo "$status"
}

random_line() {
    local timestamp ip method url status response_time

    timestamp=$(date "+%Y-%m-%d %H:%M:%S")
    ip=$(random_ip)
    IFS=',' read -r method url <<< "$(random_request)"
    status=$(random_status "$method" "$url")
    response_time=$((RANDOM % 1991 + 10))

    echo "$timestamp,$ip,$method,$url,$status,$response_time"
}


# main
if [[ ! -f "$FILE_PATH" || ! -s "$FILE_PATH" ]]; then
  echo "timestamp,ip,method,url,status,response_time" > "$FILE_PATH"
fi

COUNT="${MODE_SPECIFIC_ARG}"
for ((i = 0; i < COUNT; i++)); do
  random_line >> "$FILE_PATH"
  if (( (i + 1) % 1000 == 0 || i + 1 == COUNT )); then
    show_progress $((i + 1)) "$COUNT"
  fi
done

echo "Done. $COUNT log lines added to $FILE_PATH"
