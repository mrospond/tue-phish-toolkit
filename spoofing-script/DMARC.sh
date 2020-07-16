#!/bin/bash

# delete old files
if test -f "no-DMARC.txt"; then rm "no-DMARC.txt"; fi
if test -f "domains.txt"; then rm "domains.txt"; fi
if test -f "quarantine.txt"; then rm "quarantine.txt"; fi
if test -f "reject.txt"; then rm "reject.txt"; fi
if test -f "no-answer.txt"; then rm "no-answer.txt"; fi
	
# type of asnwers
isnt="server can't find"
none="p=none"
quarantine="p=quarantine"
reject="p=reject"
	
# read 'out.csv' from the third line
n=0
while IFS="," read -r fuzzer domain rest
do
	if [[ $n -gt 1 ]]
	then
		# query DNS server for DMARC record
		query=$(nslookup -type=txt _dmarc.${domain})
		
		if echo "$query" | grep -q "$isnt"
		then
			#echo "DMARC record not present or website not registered"
			echo "$domain" >> no-DMARC.txt
		elif echo "$query" | grep -q "$none"
		then
			#echo "p=none"
			echo "$domain" >> domains.txt
		elif echo "$query" | grep -q "$quarantine"
		then
			#echo "p=quarantine"
			echo "$domain" >> quarantine.txt
		elif echo "$query" | grep -q "$reject"
		then
			#echo "p=reject"
			echo "$domain" >> reject.txt
		else 
			#echo "no-answer"
			echo "$domain" >> no-answer.txt
		fi
	else 
		n=$((${n}+1))
	fi
done < out.csv

