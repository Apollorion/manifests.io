import main
import json

f = open("./lambda@edgefunction/testevent.json", "r")
request = json.loads(f.read())
f.close()

response = main.handler(request, None)

print("Lambda Response", response)
