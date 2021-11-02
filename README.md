# A Toolkit for Tailored Phishing

The toolkit allows the deployment of sophisticated, tailored phishing campaigns at scale. It comprises two components:

- An extension of [Gophish](https://github.com/gophish/gophish) for the specification of highly customizable phishing email templates
- A bash script for the selection of credible phishing domains.

## Gophish extension

The toolkit extends [Gophish](https://github.com/gophish/gophish) (v. 0.9.0) to enable the design of fully flexible email templates that can be instantiated with available (OSINT) information on the recipient. The setup allows the definition of a default condition that triggers when no value specific to a subject’s variable is found.

### Install

1. Download and extract the archive containing the binary:

```
https://gitlab.com/Pirocca/gophish-extension-x64.zip
```

Create a new SSL certificate and Private Keys (needed for HTTPS communications):

```
$ openssl req -newkey rsa:2048 -nodes -keyout gophish.key -x509 -days 365 -out gophish.crt
```

Move gophish.crt and gophish.key files into the $GOPHISH_PATH directory:

```
$ mv gophish.crt $GOPHISH_PATH
$ mv gophish.key $GOPHISH_PATH
```

Restore the original database (if needed) by deleting $GOPHISH_PATH/gophish.db

### Building From Source

**If you are building from source, please note that Gophish requires Go v1.10 or above!**

Download repository:

```
$ git clone https://gitlab.com/Pirocca/phish-toolkit.git
```

Download the Go archive based on the OS type (needed to compile source files):

```
https://golang.org/dl/
```

Extract the archive in /usr/local, for example:

```
$ tar -C /usr/local -xzf go1.2.1.linux-amd64.tar.gz
```

Add the environment variable, inserting the following line in /etc/profile:

```
export PATH=$PATH:/usr/local/go/bin
```

Build the Gophish extension:

```
$ cd $GOPHISH_PATH
$ go build
```

### Run

1. Start the server:

```
$ cd $GOPHISH_PATH
$ sudo ./gophish
```

Navigate to https://server_ip:3333

Log in as administrator (Username: admin – Password: gophish)

## Spoofing script

The script requires the target domain name as input and checks the DMARC record of that domain. The check is done by querying the DNS using [nslookup](https://linux.die.net/man/1/nslookup). Domain names resembling the target domain are generated using [dnstwist](https://github.com/elceef/dnstwist).

### Install & Run

Install Python3 dependencies:

```
$ sudo apt install python3-dnspython python3-tldextract python3-geoip python3-whois python3-requests python3-ssdeep python3-pip
```

Install dnstwist (requires python version at least 3.7):

```
$ git clone https://github.com/elceef/dnstwist.git
$ cd dnstwist
$ pip3 install .
```

Download repository from Git:

```
$ git clone https://gitlab.com/Pirocca/phish-toolkit.git
```

Go to $SPOOF_PATH and execute:

```
$ bash spoofing_script.sh
```

## Additional information

- https://docs.getgophish.com/user-guide/installation
- https://docs.getgophish.com/user-guide/getting-started
- https://golang.org/doc/install
- https://github.com/elceef/dnstwist

### License

```
A Toolkit for Tailored Phishing

The MIT License (MIT)

Copyright (c) 2020

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
```
