#!/bin/sh
# Copyright (C) The Arvados Authors. All rights reserved.
#
# SPDX-License-Identifier: Apache-2.0

set -x

cwldir=$(readlink -f $(dirname $0))

if ! which arvbox >/dev/null ; then
    export PATH=$PATH:$cwldir/../../tools/arvbox/bin
fi

reset_container=1
leave_running=0
config=dev
devcwl=0
tag="latest"
suite=conformance
runapi=containers
reinstall=0

while test -n "$1" ; do
    arg="$1"
    case "$arg" in
        --no-reset-container)
            reset_container=0
            shift
            ;;
        --leave-running)
            leave_running=1
            shift
            ;;
        --config)
            config=$2
            shift ; shift
            ;;
        --tag)
            tag=$2
            shift ; shift
            ;;
        --build)
            build=1
            shift
            ;;
        --devcwl)
            devcwl=1
            shift
            ;;
        --reinstall)
            reinstall=1
            shift
            ;;
        --pythoncmd)
            echo "warning: --pythoncmd option is no longer supported; ignored" >&2
            shift ; shift
            ;;
        --suite)
            suite=$2
            shift ; shift
            ;;
	--api)
	    runapi=$2
            shift ; shift
            ;;
        -h|--help)
            echo "$0 [--no-reset-container] [--leave-running] [--config dev|localdemo] [--tag docker_tag] [--build] [--suite (integration|conformance-v1.0|conformance-*)]"
            exit
            ;;
        *)
            break
            ;;
    esac
done

if test -z "$ARVBOX_CONTAINER" ; then
   export ARVBOX_CONTAINER=cwltest
fi

if test "$suite" = "conformance" ; then
  suite=conformance-v1.0
fi

if test $reset_container = 1 ; then
    arvbox stop
    docker rm $ARVBOX_CONTAINER
    arvbox reset -f
fi

arvbox start $config $tag

githead=$(git rev-parse HEAD)

arvbox pipe <<EOF
set -eu -o pipefail

. /usr/local/lib/arvbox/common.sh

# Switch to the branch that the outer script is running from,
# this ensures we get the right version of tests and a-c-r
cd /usr/src/arvados
git config --global --add safe.directory /usr/src/arvados
git fetch -a
git checkout -f $githead

if test $config = dev -o $reinstall = 1; then
  pip_install_sdist sdk/python sdk/cwl
fi

set -x

# Starting with the 2.5.20241122133319 release of cwltest, it looks
# for an entry point declared as cwltest.fsaccess and will use that
# for file access.  As of 2024-11-25 arvados-cwl-runner publishes this
# entry point so that cwltest uses CollectionFsAccess, allowing cwltest to
# check that files exist in Keep.
/opt/arvados-py/bin/pip install 'cwltest >=2.5.20241122133319,<3'

mkdir -p /tmp/cwltest
cd /tmp/cwltest

if [[ "$suite" = "conformance-v1.0" ]] ; then
   if ! test -d common-workflow-language ; then
     git clone https://github.com/common-workflow-language/common-workflow-language.git
   fi
   cd common-workflow-language
elif [[ "$suite" =~ conformance-(.*) ]] ; then
   version=\${BASH_REMATCH[1]}
   if ! test -d cwl-\${version} ; then
     git clone https://github.com/common-workflow-language/cwl-\${version}.git
   fi
   cd cwl-\${version}
   git checkout \${version}.0
elif [[ "$suite" != "integration" ]] ; then
   echo "ERROR: unknown suite '$suite'"
   exit 1
fi

if [[ "$suite" = "conformance-v1.1" ]] ; then
   git checkout main
fi

if [[ "$suite" = "conformance-v1.2" ]] ; then
   git checkout v1.2.1
fi

#if [[ "$suite" != "integration" ]] ; then
#  git pull
#fi

export ARVADOS_API_HOST=localhost:8000
export ARVADOS_API_HOST_INSECURE=1
export ARVADOS_API_TOKEN=\$(cat /var/lib/arvados-arvbox/superuser_token)

if test -n "$build" ; then
  /usr/src/arvados/build/build-dev-docker-jobs-image.sh
fi

EXTRA=--compute-checksum

if [[ $devcwl -eq 1 ]] ; then
   EXTRA="\$EXTRA --enable-dev"
fi

env

arvados-cwl-runner --version
cwltest --version

# Skip docker_entrypoint test because it fails on singularity
#
# Skip timelimit_invalid_wf test because the timeout is very short
# (5s) and singularity containers loading off an arv-mount take too long
# to start and get incorrectly terminated
#
# Skip test 199 in the v1.1 suite because it has different output
# depending on whether there is a pty associated with stdout (fixed in
# the v1.2 suite)

if [[ "$suite" = "integration" ]] ; then
   cd /usr/src/arvados/sdk/cwl/tests
   exec ./arvados-tests.sh $@
elif [[ "$suite" = "conformance-v1.2" ]] ; then
   exec cwltest --tool arvados-cwl-runner --test conformance_tests.yaml -Sdocker_entrypoint --badgedir /tmp/badges $@ -- \$EXTRA
elif [[ "$suite" = "conformance-v1.1" ]] ; then
   exec cwltest --tool arvados-cwl-runner --test conformance_tests.yaml -Sdocker_entrypoint,timelimit_invalid_wf -N199 --badgedir /tmp/badges $@ -- \$EXTRA
elif [[ "$suite" = "conformance-v1.0" ]] ; then
   exec cwltest --tool arvados-cwl-runner --test v1.0/conformance_test_v1.0.yaml -Sdocker_entrypoint --badgedir /tmp/badges $@ -- \$EXTRA
fi
EOF

CODE=$?

docker cp -L $ARVBOX_CONTAINER:/tmp/badges $cwldir/badges

if test $leave_running = 0 ; then
    arvbox stop
fi

exit $CODE
