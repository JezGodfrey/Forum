# Build the image with the following command in console
# docker image build -t forum .
# Then run the image with the following command in console
# docker container run -p 8080:8080 forum:latest

# Start your image with a node base image
FROM golang:1.22

# Labels to add metadata
LABEL version="1.0" description="A forum to discuss video games and other topics!"

# Make app directory, copy all files to it and run from app
RUN mkdir /app 
ADD . /app/ 
WORKDIR /app

# Run command to create the build
RUN go build -o server

# Run the built application
CMD [ "/app/server" ]