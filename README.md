# Boomerang
Boomerang is a tool to expose multiple internal servers to web/cloud. The Server will expose 2 ports on the Cloud. One will be where tools like proxychains can connect over socks, another will be for the agent to connect. The agent will be executed on any internal host. The agent will connect to the server and listen for any connection that can be forwarded to internal machine like a socks server. A more detailed information can be found in the image below. Features like authentication are in pipeline and will be added soon. <br/>
Agent & Server are pretty stable and can be used for Red Team Multiple level Pivotting<br/><br/>

![Alt text](docs/Boomerang_v0.1.png?raw=true "Boomerang")

<br/>
Features in Progress:
Proxy Authentication (Use IP Whitelisting for C2s till then)
