#!/bin/bash
#
# MinIO Cloud Storage, (C) 2017, 2018 MinIO, Inc.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#

set -e
set -E
set -o pipefail

if [ ! -x "$PWD/bind-store" ]; then
    echo "bind-store executable binary not found in current directory"
    exit 1
fi

WORK_DIR="$PWD/.verify-$RANDOM"

export MINT_MODE=core
export MINT_DATA_DIR="$WORK_DIR/data"
export SERVER_ENDPOINT="127.0.0.1:9000"
export ACCESS_KEY="minio"
export SECRET_KEY="minio123"
export ENABLE_HTTPS=0
export GO111MODULE=on
export GOGC=25
export MINIO_CI_CD=1
export MINIO_ROOT_USER="$ACCESS_KEY"
export MINIO_ROOT_PASSWORD="$SECRET_KEY"
export MC_HOST_verify="http://${ACCESS_KEY}:${SECRET_KEY}@${SERVER_ENDPOINT}/"
export MC_HOST_verify_ipv6="http://${ACCESS_KEY}:${SECRET_KEY}@[::1]:9000/"

MINIO_CONFIG_DIR="$WORK_DIR/.minio"
MINIO=( "$PWD/bind-store" --config-dir "$MINIO_CONFIG_DIR" )

FILE_1_MB="$MINT_DATA_DIR/datafile-1-MB"
FILE_65_MB="$MINT_DATA_DIR/datafile-65-MB"

FUNCTIONAL_TESTS="$WORK_DIR/functional-tests.sh"

# Legacy MinIO admin API uses set-user-or-group-policy; mc master uses policy attach.
# Pin mc to a release whose functional-tests still use `admin policy set`.
MC_RELEASE="${MC_RELEASE:-RELEASE.2022-05-09T04-08-26Z}"

function wait_minio_ready() {
    local alias="${1:-verify}"
    if "${WORK_DIR}/mc" ready --insecure "${alias}" 2>/dev/null; then
        return 0
    fi
    # Older mc releases have no 'ready' subcommand; poll health instead.
    local i=0
    while [ "$i" -lt 60 ]; do
        if curl -sf "http://${SERVER_ENDPOINT}/minio/health/live" >/dev/null 2>&1; then
            return 0
        fi
        i=$((i + 1))
        sleep 1
    done
    return 1
}

function install_mc_and_tests() {
    local mc_build_dir="mc-$RANDOM"
    echo "fetching mc ${MC_RELEASE} (functional-tests)..."
    mkdir -p "${mc_build_dir}"
    if ! (cd "${mc_build_dir}" && git init -q . && git remote add origin https://github.com/minio/mc && git fetch --depth 1 origin tag "${MC_RELEASE}" && git checkout -q FETCH_HEAD); then
        echo "failed to clone https://github.com/minio/mc at ${MC_RELEASE}"
        purge "${mc_build_dir}"
        exit 1
    fi

    local mc_os mc_arch mc_url
    mc_os=$(go env GOOS)
    mc_arch=$(go env GOARCH)
    mc_url="https://dl.min.io/client/mc/release/${mc_os}-${mc_arch}/archive/mc.${MC_RELEASE}"
    echo "downloading mc binary from ${mc_url}..."
    if curl -sfL --connect-timeout 30 --max-time 300 -o "$WORK_DIR/mc" "$mc_url" || wget -q -T 30 -O "$WORK_DIR/mc" "$mc_url"; then
        chmod +x "$WORK_DIR/mc"
    elif command -v docker >/dev/null 2>&1 && docker pull -q "minio/mc:${MC_RELEASE}" >/dev/null 2>&1; then
        echo "downloading mc binary failed, extracting from minio/mc:${MC_RELEASE} docker image"
        local mc_cid="mc-extract-$$"
        docker create --name "${mc_cid}" "minio/mc:${MC_RELEASE}" >/dev/null
        docker cp "${mc_cid}:/usr/bin/mc" "$WORK_DIR/mc"
        docker rm "${mc_cid}" >/dev/null
        chmod +x "$WORK_DIR/mc"
    else
        echo "downloading mc binary failed, building ${MC_RELEASE} from source"
        rm -f "${mc_build_dir}/go.sum"
        if ! (cd "${mc_build_dir}" && go build -mod=mod -o "$WORK_DIR/mc"); then
            purge "${mc_build_dir}"
            exit 1
        fi
    fi

    cp "${mc_build_dir}/functional-tests.sh" "$FUNCTIONAL_TESTS"
    purge "${mc_build_dir}"
    echo "mc ${MC_RELEASE} installed"
}

function start_minio_fs()
{
    export MINIO_ROOT_USER=$ACCESS_KEY
    export MINIO_ROOT_PASSWORD=$SECRET_KEY
    "${MINIO[@]}" server --address 127.0.0.1:9000 "${WORK_DIR}/fs-disk" >"$WORK_DIR/fs-minio.log" 2>&1 &
    wait_minio_ready verify
    sleep 10
}

function start_minio_erasure()
{
    export MINIO_ROOT_USER=$ACCESS_KEY
    export MINIO_ROOT_PASSWORD=$SECRET_KEY
    "${MINIO[@]}" server --address 127.0.0.1:9000 "${WORK_DIR}/erasure-disk1" "${WORK_DIR}/erasure-disk2" "${WORK_DIR}/erasure-disk3" "${WORK_DIR}/erasure-disk4" >"$WORK_DIR/erasure-minio.log" 2>&1 &
    wait_minio_ready verify
    sleep 15
}

function start_minio_erasure_sets()
{
    export MINIO_ROOT_USER=$ACCESS_KEY
    export MINIO_ROOT_PASSWORD=$SECRET_KEY
    export MINIO_ENDPOINTS="${WORK_DIR}/erasure-disk-sets{1...32}"
    "${MINIO[@]}" server --address 127.0.0.1:9000 > "$WORK_DIR/erasure-minio-sets.log" 2>&1 &
    wait_minio_ready verify
    sleep 15
}

function start_minio_pool_erasure_sets()
{
    export MINIO_ROOT_USER=$ACCESS_KEY
    export MINIO_ROOT_PASSWORD=$SECRET_KEY
    export MINIO_ENDPOINTS="http://127.0.0.1:9000${WORK_DIR}/pool-disk-sets{1...4} http://127.0.0.1:9001${WORK_DIR}/pool-disk-sets{5...8}"
    "${MINIO[@]}" server --address 127.0.0.1:9000 > "$WORK_DIR/pool-minio-9000.log" 2>&1 &
    "${MINIO[@]}" server --address 127.0.0.1:9001 > "$WORK_DIR/pool-minio-9001.log" 2>&1 &
    wait_minio_ready verify
    sleep 40
}

function start_minio_pool_erasure_sets_ipv6()
{
    export MINIO_ROOT_USER=$ACCESS_KEY
    export MINIO_ROOT_PASSWORD=$SECRET_KEY
    export MINIO_ENDPOINTS="http://[::1]:9000${WORK_DIR}/pool-disk-sets{1...4} http://[::1]:9001${WORK_DIR}/pool-disk-sets{5...8}"
    "${MINIO[@]}" server --address="[::1]:9000" > "$WORK_DIR/pool-minio-ipv6-9000.log" 2>&1 &
    "${MINIO[@]}" server --address="[::1]:9001" > "$WORK_DIR/pool-minio-ipv6-9001.log" 2>&1 &
    wait_minio_ready verify_ipv6
    sleep 40
}

function start_minio_dist_erasure()
{
    export MINIO_ROOT_USER=$ACCESS_KEY
    export MINIO_ROOT_PASSWORD=$SECRET_KEY
    export MINIO_ENDPOINTS="http://127.0.0.1:9000${WORK_DIR}/dist-disk1 http://127.0.0.1:9001${WORK_DIR}/dist-disk2 http://127.0.0.1:9002${WORK_DIR}/dist-disk3 http://127.0.0.1:9003${WORK_DIR}/dist-disk4"
    for i in $(seq 0 3); do
        "${MINIO[@]}" server --address 127.0.0.1:900${i} > "$WORK_DIR/dist-minio-900${i}.log" 2>&1 &
    done
    wait_minio_ready verify
    sleep 40
}

function run_test_fs()
{
    start_minio_fs

    (cd "$WORK_DIR" && "$FUNCTIONAL_TESTS") | tee "$WORK_DIR/functional-tests.log"
    rv=${PIPESTATUS[0]}

    pkill bind-store || true
    sleep 3

    if [ "$rv" -ne 0 ]; then
        echo "functional-tests output:"
        cat "$WORK_DIR/functional-tests.log"
        echo "minio server log:"
        cat "$WORK_DIR/fs-minio.log"
    fi
    rm -f "$WORK_DIR/fs-minio.log" "$WORK_DIR/functional-tests.log"

    return "$rv"
}

function run_test_erasure_sets()
{
    start_minio_erasure_sets

    (cd "$WORK_DIR" && "$FUNCTIONAL_TESTS") | tee "$WORK_DIR/functional-tests.log"
    rv=${PIPESTATUS[0]}

    pkill bind-store || true
    sleep 3

    if [ "$rv" -ne 0 ]; then
        echo "functional-tests output:"
        cat "$WORK_DIR/functional-tests.log"
        echo "minio server log:"
        cat "$WORK_DIR/erasure-minio-sets.log"
    fi
    rm -f "$WORK_DIR/erasure-minio-sets.log" "$WORK_DIR/functional-tests.log"

    return "$rv"
}

function run_test_pool_erasure_sets()
{
    start_minio_pool_erasure_sets

    (cd "$WORK_DIR" && "$FUNCTIONAL_TESTS") | tee "$WORK_DIR/functional-tests.log"
    rv=${PIPESTATUS[0]}

    pkill bind-store || true
    sleep 3

    if [ "$rv" -ne 0 ]; then
        echo "functional-tests output:"
        cat "$WORK_DIR/functional-tests.log"
        for i in $(seq 0 1); do
            echo "server$i log:"
            cat "$WORK_DIR/pool-minio-900$i.log"
        done
    fi

    for i in $(seq 0 1); do
        rm -f "$WORK_DIR/pool-minio-900$i.log"
    done
    rm -f "$WORK_DIR/functional-tests.log"

    return "$rv"
}

function run_test_pool_erasure_sets_ipv6()
{
    start_minio_pool_erasure_sets_ipv6

    export SERVER_ENDPOINT="[::1]:9000"

    (cd "$WORK_DIR" && "$FUNCTIONAL_TESTS") | tee "$WORK_DIR/functional-tests.log"
    rv=${PIPESTATUS[0]}

    export SERVER_ENDPOINT="127.0.0.1:9000"

    pkill bind-store || true
    sleep 3

    if [ "$rv" -ne 0 ]; then
        echo "functional-tests output:"
        cat "$WORK_DIR/functional-tests.log"
        for i in $(seq 0 1); do
            echo "server$i log:"
            cat "$WORK_DIR/pool-minio-ipv6-900$i.log"
        done
    fi

    for i in $(seq 0 1); do
        rm -f "$WORK_DIR/pool-minio-ipv6-900$i.log"
    done
    rm -f "$WORK_DIR/functional-tests.log"

    return "$rv"
}

function run_test_erasure()
{
    start_minio_erasure

    (cd "$WORK_DIR" && "$FUNCTIONAL_TESTS") | tee "$WORK_DIR/functional-tests.log"
    rv=${PIPESTATUS[0]}

    pkill bind-store || true
    sleep 3

    if [ "$rv" -ne 0 ]; then
        echo "functional-tests output:"
        cat "$WORK_DIR/functional-tests.log"
        echo "minio server log:"
        cat "$WORK_DIR/erasure-minio.log"
    fi
    rm -f "$WORK_DIR/erasure-minio.log" "$WORK_DIR/functional-tests.log"

    return "$rv"
}

function run_test_dist_erasure()
{
    start_minio_dist_erasure

    (cd "$WORK_DIR" && "$FUNCTIONAL_TESTS") | tee "$WORK_DIR/functional-tests.log"
    rv=${PIPESTATUS[0]}

    pkill bind-store || true
    sleep 3

    if [ "$rv" -ne 0 ]; then
        echo "functional-tests output:"
        cat "$WORK_DIR/functional-tests.log"
        echo "server1 log:"
        cat "$WORK_DIR/dist-minio-9000.log"
        echo "server2 log:"
        cat "$WORK_DIR/dist-minio-9001.log"
        echo "server3 log:"
        cat "$WORK_DIR/dist-minio-9002.log"
        echo "server4 log:"
        cat "$WORK_DIR/dist-minio-9003.log"
    fi

    rm -f "$WORK_DIR/dist-minio-9000.log" "$WORK_DIR/dist-minio-9001.log" "$WORK_DIR/dist-minio-9002.log" "$WORK_DIR/dist-minio-9003.log" "$WORK_DIR/functional-tests.log"

    return "$rv"
}

function purge()
{
    rm -rf "$1"
}

function __init__()
{
    echo "Initializing environment"
    mkdir -p "$WORK_DIR"
    mkdir -p "$MINIO_CONFIG_DIR"
    mkdir -p "$MINT_DATA_DIR"

    install_mc_and_tests

    shred -n 1 -s 1M - 1>"$FILE_1_MB" 2>/dev/null
    shred -n 1 -s 65M - 1>"$FILE_65_MB" 2>/dev/null

    ## version is purposefully set to '3' for minio to migrate configuration file
    echo '{"version": "3", "credential": {"accessKey": "minio", "secretKey": "minio123"}, "region": "us-east-1"}' > "$MINIO_CONFIG_DIR/config.json"

    sed -i 's|-sS|-sSg|g' "$FUNCTIONAL_TESTS"
    chmod a+x "$FUNCTIONAL_TESTS"
}

function main()
{
    echo "Testing in FS setup"
    if ! run_test_fs; then
        echo "FAILED"
        purge "$WORK_DIR"
        exit 1
    fi

    echo "Testing in Erasure setup"
    if ! run_test_erasure; then
        echo "FAILED"
        purge "$WORK_DIR"
        exit 1
    fi

    echo "Testing in Distributed Erasure setup"
    if ! run_test_dist_erasure; then
        echo "FAILED"
        purge "$WORK_DIR"
        exit 1
    fi

    echo "Testing in Erasure setup as sets"
    if ! run_test_erasure_sets; then
        echo "FAILED"
        purge "$WORK_DIR"
        exit 1
    fi

    echo "Testing in Distributed Eraure expanded setup"
    if ! run_test_pool_erasure_sets; then
        echo "FAILED"
        purge "$WORK_DIR"
        exit 1
    fi

    echo "Testing in Distributed Erasure expanded setup with ipv6"
    if ! run_test_pool_erasure_sets_ipv6; then
        echo "FAILED"
        purge "$WORK_DIR"
        exit 1
    fi

    purge "$WORK_DIR"
}

( __init__ "$@" && main "$@" )
rv=$?
purge "$WORK_DIR"
exit "$rv"
