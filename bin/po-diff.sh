#!/usr/bin/env bash

# Copyright 2019 Pressinfra SRL
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

set -o errexit
set -o nounset
set -o pipefail

export ROOT_DIR=$(dirname "${BASH_SOURCE}")/../..

# Install tools we need, but only from vendor/...
cd "${ROOT_DIR}"

diff -u \
    <(grep -E '^msgid' "${1}" | sort | sed 's/msgid[[:space:]]*//g') \
    <(grep -E '^msgid' "${2}" | sort | sed 's/msgid[[:space:]]*//g')

exit 0
