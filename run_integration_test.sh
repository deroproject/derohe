#!/usr/bin/env bash

CURDIR=`/bin/pwd`
BASEDIR=$(dirname $0)
ABSPATH=$(readlink -f $0)
ABSDIR=$(dirname $ABSPATH)


# set -x   # to enable debug and verbose printing of each and every command
_DEBUG="on"
function DEBUG()
{
 [ "$_DEBUG" == "on" ] && $@
}


function check_errs()
{
  # Function. Parameter 1 is the return code
  # Para. 2 is text to display on failure.
  
  if [ "${1}" -ne "0" ]; then
    echo "ERROR # ${1} : ${2}"

#    if [ "$#" -eq "3"]; then
    	echo "logs "
    	cat /dev/shm/dtest/log.txt
#    	cat "${3}"
#    fi
    # as a bonus, make our script exit with the right error code.
#    rm -rf /dev/shm/dtest >/dev/null 2>&1
    killall -9 simulator >/dev/null 2>&1
    exit ${1}
  fi
}


function run_single_test()
{
test=${1} 
	killall -9 simulator >/dev/null 2>&1
	rm -rf /dev/shm/dtest >/dev/null 2>&1
	mkdir /dev/shm/dtest
	cd  /dev/shm/dtest

	/tmp/simulator  --clog-level 2  $disableautomine  >/dev/shm/dtest/dero.log 2>&1 &
	disown
	check_errs $? "Could not run simulator"
   	echo "Simulator started"	
	sleep 4

	DEBUG echo "running test $test"
	rm /dev/shm/dtest/log.txt  >/dev/null 2>&1
    rm /dev/shm/dtest/log.txt  >/dev/null 2>&1

	cd $test
	$test/run_test.sh >/dev/shm/dtest/log.txt 2>&1 

	check_errs $? "test failed $test" 
	killall -9 simulator >/dev/null 2>&1
	cd $CURDIR
}

function run_tests()
{
	tests=$(find ${1} -mindepth 1 -maxdepth 1 -type d)
	for test in $tests; do 
		run_single_test "$test"
	done
}

cd $ABSDIR
go build   -o /tmp/ ./cmd/...
check_errs $? "Could not build binaries"
cd $CURDIR



 if [[ $# -eq 1 ]];  then   # if user requsted single test

 	if [ -d "$ABSDIR/tests/normal/${1}" ]; then
 	run_single_test "$ABSDIR/tests/normal/${1}"
 	exit 0
 	fi

 	if [ -d "$ABSDIR/tests/specific/${1}" ]; then
 	disableautomine="--noautomine"
 	run_single_test "$ABSDIR/tests/specific/${1}"
 	exit 0
 	fi

 	echo "no such test found ${1}"
 	exit 0  	 
 fi

#we will first run specific/special test cases
disableautomine="--noautomine"
run_tests $ABSDIR/tests/specific

#we will first run normal test cases
disableautomine=""
run_tests $ABSDIR/tests/normal

