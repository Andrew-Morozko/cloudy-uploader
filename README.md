# Cloudy Uploader
*Unofficial CLI client for [Overcast](https://overcast.fm/)® Premium upload feature*

It's just a wrapper around upload a form at [https://overcast.fm/uploads](https://overcast.fm/uploads).

I wanted to automate the uploading of my mp3s to my favorite podcast player and decided to share it here. Sorry for the code quality, this is also my first go project. It works, whatever;) Issues and pull requests are welcome.

This project shouldn't cause any trouble, but I (of course) will shut it down if Marco isn't ok with it.

```
Usage: cloudyuploader [--login LOGIN] [--password PASSWORD]
                      [--save-creds] [--no-load-creds] [--silent]
                      [--parallel-uploads N] [--unordered-submit] FILE [FILE ...]

Positional arguments:
  FILE                   files to be uploaded

Options:
  --login LOGIN          email for Overcast account
  --password PASSWORD    password for Overcast account
  --save-creds           save credentials in secure system storge
  --no-load-creds        do not use stored creds
  --parallel-uploads N, -j N
                         maximum number of concurrent uploads [default: 4]
  --silent, -s           disable user interaction
  --unordered-submit     don't wait to submit uploads in proper order
  --help, -h             display this help and exit
```

## macOS first launch note
You need to run `xattr -rc ./cloudyuploader` before launching the tool. Otherwise apple will helpfully suggest throwing the app in the trash since I'm not an Identified Developer. Still figuring out how to deal with this without paying apple $100/year, code signatures are a mess.

## Authentication

There are 3 ways to supply `cloudyuploader` with credentials. In order of priority:
1. `--login` and `--password` command line arguments
2. Credentials saved in secure system storage (macOS Keychain, GNOME Keyring or Windows Credential Manager API)
3. Enter them interactively, when prompted

You can prevent any of these methods:
1. Don't supply `--login` and `--password` command line arguments
2. Add `--no-load-creds`
3. Add `--silent` (also disables all output, useful for scripting/automation)

If you want to save the credentials to secure system storage:
* Add `--save-creds` command line argument
* You will be asked to save the credentials after a successful interactive login

### Note for macOS users

In order to save or read the credentials from the keychain it should be unlocked. Normally unlocking your mac is all that's needed, however, this doesn't extend to ssh logins. If you are connecting over ssh you should execute `security unlock-keychain` command prior to running `cloudyuploader`.

`security unlock-keychain` could be automated, it accepts `-p PASSWORD` argument, but storing your keychain's (mac's) password in this way is really insecure.

Your best bet is to supply Overcast `--login` and `--password` via the command line, they are less important than your main keychain password.
