docker buildx build --push --tag jhash14/tabitha:latest . && ssh deploy@$OCI_SERVER_IP 'docker pull jhash14/tabitha:latest && docker stack deploy -c /home/deploy/stacks/tabitha/stack.yml tabitha'
