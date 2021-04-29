## Purpose

Exercise my go skills, build something fun that I can use every day. I have to
join a meet session every day. Why do I have to click a link? That is annoying
to do every day. So why not automate it?




## Features
* Makefile to build consistently in a local environment and remote environment
* Dockerfile for a generic image to build for 
* Go Mod (which you should to your project path change)
* VS Code environment
* Generic docker push
* Chromedp browser launch
* Google Login Detection
* Meeting Launch Automation
* Google Meet Support
* Client that grabs calendar events
* GRPC Server that opens the browser and launches the meeting

## TODO 
* ~~build a grpc service that opens up a meet session~~
* ~~build a  service that reads the calendar~~
* ~~Make a grpc service for calendar and meet joining~~
* ~~Make a daemon that looks for new calendar events to autojoin~~
* ~~REFACTOR~~
* Seperate auth into a different service that gives oauth2 flow but browser grabs the token
* build a front end system in Dash
* make portable to share with people from a 1 click install
* support over the wire upgrades



## Installing via brew
* `brew install --verbose --build-from-source brew/Formula/go-grpc-video-call-manager.rb`
