kubectl patch service --namespace ingress-nginx ingress-nginx -p '{"spec":{"loadBalancerIP": "35.231.159.109"}}'
kubectl apply -f .