# Wordlistctl

## Description 

wordlistctl - Fetch, install and search wordlist archives from websites.
Script to fetch, install, update and search wordlist archives from websites
offering wordlists with more than 6300 wordlists available.

## Installation

On Debian-based distros, add my ppa:

```bash
curl -s --compressed "https://ppa.casalino.xyz/KEY.gpg" | sudo apt-key add -
sudo curl -s --compressed -o /etc/apt/sources.list.d/my_list_file.list "https://ppa.casalino.xyz/my_list_file.list"
sudo apt update
sudo apt install wordlistctl
```

More are coming...

## Notes 

This is the first time ever that I'm using Go, so please be kind and make
contributions if you think that something could have been done better,
I'm eager to learn! Thank you!!

## LICENSE

GPLv3, this is a derived work from the Black Arch team, more specifically,
as stated in the main document, from sepehrdad@blackarch.org
====> [https://github.com/BlackArch/wordlistctl](https://github.com/BlackArch/wordlistctl)
