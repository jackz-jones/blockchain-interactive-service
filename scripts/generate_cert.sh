
out_path=$1
conf_path=$2

# 1、生成 ca 秘钥

openssl genrsa -out ${out_path}/ca.key 4096

# 2、生成 ca 证书签发请求

openssl req -new -sha256 -out ${out_path}/ca.csr -key ${out_path}/ca.key -config ${conf_path}/ca.conf -batch

# 3、生成 ca 根证书

openssl x509 -req -days 3650 -in ${out_path}/ca.csr -signkey ${out_path}/ca.key -out ${out_path}/ca.pem

# 4、生成 server 端秘钥

openssl genrsa -out ${out_path}/server.key 2048

# 5、生成 server 端证书签发请求

openssl req -new -sha256 -out ${out_path}/server.csr -key ${out_path}/server.key -config ${conf_path}/server.conf -batch

# 6、生成 server 端证书

openssl x509 -req -days 3650 -CA ${out_path}/ca.pem -CAkey ${out_path}/ca.key -CAcreateserial -in ${out_path}/server.csr -out ${out_path}/server.pem -extensions req_ext -extfile ${conf_path}/server.conf

# 7. 生成 client 端秘钥

openssl ecparam -genkey -name secp384r1 -out ${out_path}/client.key

# 8. 生成 client 端证书签发请求

openssl req -new -sha256 -out ${out_path}/client.csr -key ${out_path}/client.key -config ${conf_path}/client.conf  -batch

# 9. 生成 client 端证书

openssl x509 -req -days 3650 -CA ${out_path}/ca.pem -CAkey ${out_path}/ca.key -CAcreateserial -in ${out_path}/client.csr -out ${out_path}/client.pem -extensions req_ext -extfile ${conf_path}/client.conf

# 10. 只保留 key 和 pem 文件，删除其他不需要的文件

rm ${out_path}/*.csr
rm ${out_path}/*.srl