# Cloudy Uploader
*Unofficial CLI client for [Overcast](https://overcast.fm/)® Premium upload feature*

It's just a wrapper around upload a form at [https://overcast.fm/uploads](https://overcast.fm/uploads).

I wanted to automate the uploading of my mp3s to my favorite podcast player and decided to share it here.
Sorry for the code quality, this is also my first go project. It works, whatever;) Issues and pull requests are welcome.

This project shouldn't cause any trouble, but I (of course) will shut it down if Marco isn't ok with it.

```
Usage: cloudyuploader [--parallel-uploads N] [--save-creds] [--silent]
                      [--login LOGIN] [--password PASSWORD] FILE [FILE ...]

Positional arguments:
  FILE                   files to be uploaded

Options:
  --parallel-uploads N, -j N
                         maximum number of concurrent upload jobs [default: 4]
  --login LOGIN          email for Overcast account
  --password PASSWORD    password for Overcast account
  --save-creds           save credentials in secure system storge [default: true]
  --silent, -s           disable user interaction
  --help, -h             display this help and exit
```
