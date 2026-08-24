# Security

## Reporting a problem

Do not open a public issue for a security problem. A public report tells
everyone how to use the flaw before there is a fix for it.

Report it through [GitHub's private vulnerability
reporting](https://github.com/Sillyfrogster/Illarin/security/advisories/new) on
this repository. If that page is not available to you, contact a maintainer
directly and ask for a private channel.

Anyone can report. You do not need an account in good standing, prior
contributions, or an invitation.

## What to send

- What you found, and where. A page address, an API route, or a file and line.
- The steps to reproduce it, in the order you did them.
- What someone gains from it. Reading another person's files, taking over an
  account, running code on the server, and so on.
- Anything that shows it working. A short script, a request, a screenshot.
- Whether you have told anyone else, and whether you plan to.

A report that only names a scanner rule, with nothing showing the flaw is real,
is hard to act on. Show the problem happening.

## What happens next

1. We confirm we received the report.
2. We tell you whether we can reproduce it and what we intend to do.
3. We fix it and tell you when the fix is available.
4. We credit you by whatever name you ask for, or leave you out if you prefer.

If a report goes quiet, send a reminder.

## Testing rules

You are welcome to look for problems. These are the limits.

- Test local copies with data you own or made for testing.
- Do not test a hosted Illarin instance without the operator's permission.
- Do not read, change, or download anyone else's account details or files.
- Stop as soon as you have confirmed a problem. Do not go further to see how far
  it reaches.
- Do not run denial of service, load, or brute force tests against a hosted
  instance.
- Do not use social engineering, phishing, or physical access against anyone
  working on the project.
- Prefer a local copy where you can. Run `make setup` on a fresh clone.

Reports that follow these rules are welcome and will not be treated as an
attack.

## Scope

In scope:

- The code in this repository, both the Go API and the Next.js site.
- Deployment and container configuration in this repository.

Out of scope:

- Flooding the site with traffic.
- Missing hardening headers, or a weak configuration score, with no path to an
  actual exploit.
- Problems in third party services Illarin talks to. Report those to whoever
  runs them.
- Findings that need an attacker to already control the user's machine or
  browser.
- Reports produced entirely by an automated scanner with no working
  demonstration.

## No bounty

There is no money for reports. Illarin is not a company and has no budget for
one. Credit is offered, and it is offered gladly.

## Disclosure

Please hold public details until a fix is available. If a fix is taking a long
time, talk to us about a date rather than setting one on your own.
