# Copyright (C) The Arvados Authors. All rights reserved.
#
# SPDX-License-Identifier: Apache-2.0

case "$TARGET" in
    debian* | ubuntu*)
        fpm_depends+=(libcurl4)
        ;;
esac
