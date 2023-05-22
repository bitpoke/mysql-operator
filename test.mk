include build/makelib/utils.mk
all:
	$(info |$(subst $(SPACE),_,$(strip one    foo bar))|)
	$(info $(call list-join,_,mysql-operator    foo bar))
.PHONY: all
