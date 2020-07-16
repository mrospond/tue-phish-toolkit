#!/bin/bash

#paramers for text color
default='\e[39m'
red='\e[31m'
green='\e[32m'
cyan='\e[36m'

#echo "Insert domain name to clone: (default 'tue.nl')"
echo "Insert domain name to clone (e.g. 'example.com'): "
read domain
if [ -z "${domain}" ]; then domain='tue.nl'; fi


echo " --> CHECKING DMARC RECORD OF '${domain}'..."
# checking DMARC record for the selected domain
query=$(nslookup -type=txt _dmarc.${domain})
if echo "$query" | grep -q "server can't find"
then
	echo "     DMARC record not present or website not registered"
elif echo "$query" | grep -q "p=none"
then
	echo "     DMARC record present with policy NONE"
elif echo "$query" | grep -q "p=quarantine"
then
	echo "     DMARC record present with policy QUARANTINE"
elif echo "$query" | grep -q "p=reject"
then
	echo "     DMARC record present with policy REJECT"
else 
	echo "     No answers from the DNS"
fi
echo ""

PS3='Select your choice from the menu: '
options=("All domains" "Only registered" "Only NOT registered" "Read last DOMAINS file" "Read last OUT file"  "Quit")
select opt in "${options[@]}"
do
    case $opt in
    "All domains")
		echo ""
		echo " --> LOOKING FOR ALL SIMILAR DOMAINS... (it may take some minutes)"
		
        python3 ../dnstwist/dnstwist.py -g -w -f csv ${domain} > out.csv
		
		echo "     DOMAINS SAVED IN ./out.csv"
		echo ""
		echo " --> LOOKING FOR DOMAINS THAT WE CAN ACTUALLY SPOOF... (by querying DNS for DMARC record)"
		
		bash DMARC.sh 
		
		echo "     DOMAINS SAVED IN ./domains.txt"
		echo ""
		;;
		
	"Only registered")
        echo ""
		echo " --> LOOKING FOR SIMILAR DOMAINS REGISTERED... (it may take some minutes)"
		
        python3 ../dnstwist/dnstwist.py -r -g -w -f csv ${domain} > out.csv
		
		echo "     DOMAINS SAVED IN ./out.csv"
		echo ""
		echo " --> LOOKING FOR DOMAINS THAT WE CAN ACTUALLY SPOOF... (by querying DNS for DMARC record)"
		
		bash DMARC.sh
		
		echo "     DOMAINS SAVED IN ./domains.txt"
		echo ""
		;;
		
	"Only NOT registered")
		echo ""
		echo " --> LOOKING FOR SIMILAR DOMAINS NOT REGISTERED... (it may take some minutes)"
		
        python3 ../dnstwist/dnstwist.py -g -w -f csv ${domain} > out-all.csv
		python3 ../dnstwist/dnstwist.py -r -g -w -f csv ${domain} > out-reg.csv
		diff out-all.csv out-reg.csv > out.csv
		rm out-all.csv
		rm out-reg.csv
		
		echo "     DOMAINS SAVED IN ./out.csv"
		echo ""
		echo " --> FORMATTING THE DOMAINS"
		
		bash format.sh
		
		echo "     DOMAINS SAVED IN ./domains.txt"
		echo ""
		;;
		
	"Read last DOMAINS file")
		if test -f "domains.txt"
		then 
			echo ""
			echo "OPENING LAST 'domains.txt'"
			echo "-----------------------------------------"
			cat domains.txt
			echo "-----------------------------------------"
			echo "FILE FINISHED"
			echo "" 
		else
			echo "NO DOMAINS FOUND"
		fi
		;;
		
	"Read last OUT file")
	
		if test -f "out.csv"
		then 
			echo ""
			echo "OPENING LAST 'out.csv'"
			echo "-----------------------------------------"
			cat out.csv
			echo "-----------------------------------------"
			echo "FILE FINISHED"
			echo ""
		else
			echo "FILE NOT FOUND" 
		fi
		;;
		
	"Quit")
        break
        ;;
		
    *) echo "Invalid option $REPLY";;
    esac
done


