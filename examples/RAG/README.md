# RAG Example

Inspired by https://docs.llamaindex.ai/en/stable/examples/usecases/10k_sub_question/

1. uv venv
2. source venv/bin/activate
3. uv pip install -r requirements.txt
4. kubectl apply -f kaito_ragengine_phi_3.yaml
5. kubectl port-forward svc/ragengine-example 8080:80
6. uv python index_10k.py
7. curl