# Purpose

    The host instance is making google service calls to the api to poll in the background. The client has to accept the API scope requests, then supply a token.


# TODO

    Remove the need for storing the credentials on the client side. We can move the calender api auth to a server and the client can store the token api call for the calendar poll, then send to the grpc server.
    

