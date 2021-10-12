# Copyright 2019 Pressinfra SRL. All rights reserved.
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

ifndef __UTILS_MAKEFILE__
__UTILS_MAKEFILE__ := included

COMMA := ,
SPACE :=
SPACE +=

# define a newline
define \n


endef

lower = $(subst A,a,$(subst B,b,$(subst C,c,$(subst D,d,$(subst E,e,$(subst F,f,$(subst G,g,$(subst H,h,$(subst I,i,$(subst J,j,$(subst K,k,$(subst L,l,$(subst M,m,$(subst N,n,$(subst O,o,$(subst P,p,$(subst Q,q,$(subst R,r,$(subst S,s,$(subst T,t,$(subst U,u,$(subst V,v,$(subst W,w,$(subst X,x,$(subst Y,y,$(subst Z,z,$1))))))))))))))))))))))))))
upper = $(subst a,A,$(subst b,B,$(subst c,C,$(subst d,D,$(subst e,E,$(subst f,F,$(subst g,G,$(subst h,H,$(subst i,I,$(subst j,J,$(subst k,K,$(subst l,L,$(subst m,M,$(subst n,N,$(subst o,O,$(subst p,P,$(subst q,Q,$(subst r,R,$(subst s,S,$(subst t,T,$(subst u,U,$(subst v,V,$(subst w,W,$(subst x,X,$(subst y,Y,$(subst z,Z,$1))))))))))))))))))))))))))
list-join = $(subst $(SPACE),$(1),$(strip $(2)))

# ====================================================================================
# Tools macros
#
# Theses macros are used to install tools in an idempotent, cache friendly way.

define tool
$(subst -,_,$(call upper,$(1))) := $$(TOOLS_BIN_DIR)/$(1)

build.tools: $$(TOOLS_BIN_DIR)/$(1)
$$(TOOLS_BIN_DIR)/$(1): $$(TOOLS_HOST_DIR)/$(1)-v$(2) |$$(TOOLS_BIN_DIR)
	@ln -sf $$< $$@
endef

# Creates a target for downloading a tool from a given url
# 1 tool, 2 version, 3 download url
define tool.download
$(call tool,$(1),$(2))

$$(TOOLS_HOST_DIR)/$(1)-v$(2): |$$(TOOLS_HOST_DIR)
	@echo ${TIME} ${BLUE}[TOOL]${CNone} installing $(1) version $(2) from $(3)
	@curl -fsSLo $$@ $(3) || $$(FAIL)
	@chmod +x $$@
	@$$(OK) installing $(1) version $(2) from $(3)
endef # tool.download

# Creates a target for downloading and unarchiving a tool from a given url
# 1 tool, 2 version, 3 download url, 4 tool path within archive, 5 tar strip components
define tool.download.tar.gz
$(call tool,$(1),$(2))

ifeq ($(4),)
$(1)_TOOL_ARCHIVE_PATH = $(1)
else
$(1)_TOOL_ARCHIVE_PATH = $(4)
endif


$$(TOOLS_HOST_DIR)/$(1)-v$(2): |$$(TOOLS_HOST_DIR)
	@echo ${TIME} ${BLUE}[TOOL]${CNone} installing $(1) version $(2) from $(3)
	@mkdir -p $$(TOOLS_HOST_DIR)/tmp-$(1)-v$(2) || $$(FAIL)
ifeq ($(5),)
	@curl -fsSL $(3) | tar -xz --strip-components=1 -C $$(TOOLS_HOST_DIR)/tmp-$(1)-v$(2) || $$(FAIL)
else
	@curl -fsSL $(3) | tar -xz --strip-components=$(5) -C $$(TOOLS_HOST_DIR)/tmp-$(1)-v$(2) || $$(FAIL)
endif
	@mv $$(TOOLS_HOST_DIR)/tmp-$(1)-v$(2)/$$($(1)_TOOL_ARCHIVE_PATH) $$@ || $(FAIL)
	@chmod +x $$@
	@rm -rf $$(TOOLS_HOST_DIR)/tmp-$(1)-v$(2)
	@$$(OK) installing $(1) version $(2) from $(3)
endef # tool.download.tar.gz


endif # __UTILS_MAKEFILE__
