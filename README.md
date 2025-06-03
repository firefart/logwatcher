# logwatcher

This program watches a file for changes and sends a notification (email, mailgun, discord, telegram and teams) when
one of the provided strings matches.

It's basically a `tail -f file | grep match` that sends every hit per notification.
This way you can watch a logfile for errors and get a notification to you if one occurs.

## config and installation

To install the systemd service you can use the `install_service.sh` helper script or simply copy
the `logwatcher.service` to the right place.

See `config.json.exampe` for an example config and adopt it to your needs.

For mailing, you can use [Mailjet](https://www.mailjet.com/) which gives you 200 free mails per day currently.

## usage

`logwatcher -config config.json`

Use `--help` for all available command line options
