#!/bin/bash
	
# delete old file
if test -f "domains.txt"; then rm "domains.txt"; fi

# read 'out.csv' 
while IFS= read -r line
do
	# consider only lines containing "."
	if [[ ${line} == *"."* ]]
	then
		array=(${line//,/ })
		
		# the domain is the second element of that line
		echo ${array[2]} >> domains.txt
	fi
done < out.csv		

