package hterm

//go:generate bash -c "LIBDOT_SEARCH_PATH=$DOLLAR(pwd)/../libapps/ [ -f ../public/hterm_all.js ] ||  ../libapps/libdot/bin/concat.sh -i ../libapps/hterm/concat/hterm_all.concat -o ../public/hterm_all.js"
