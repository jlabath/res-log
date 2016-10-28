module Generic exposing (Value(..), toJson, fromJson, decoder)

import Json.Encode as Encode
import Json.Decode as Decode
import List
import OrderedDict as Dict


{-| Our generic value containing all possible types we may encounter in json blob
-}
type Value
    = Num Float
    | Txt String
    | Bln Bool
    | Lst (List Value)
    | Dct (Dict.OrderedDict String Value)
    | Nil


{-| encode our generic value into json
-}
toJson : Value -> Encode.Value
toJson value =
    case value of
        Num num ->
            Encode.float num

        Txt text ->
            Encode.string text

        Bln fact ->
            Encode.bool fact

        Lst list ->
            Encode.list (List.map toJson list)

        Dct dict ->
            let
                convert ( key, value ) =
                    ( key, toJson value )

                pairs =
                    dict
                        |> Dict.toList
                        |> List.reverse
                        |> List.map convert
            in
                Encode.object pairs

        Nil ->
            Encode.null


{-| decode our generic value from json
-}
fromJson : Decode.Value -> Result String Value
fromJson blob =
    Decode.decodeValue decoder blob



{-
   So declaring recursive decoders like this breaks things
   background and workaround in detail

   https://github.com/elm-lang/core/issues/361
   https://gist.github.com/jamesmacaulay/ea28b7fdb108d53ddeb7#file-anydecoderfixed-elm

   root problem
   https://github.com/elm-lang/elm-compiler/issues/873

   workaround is to protect the recursive decoders with lambdas
   to make them lazy
-}


buildDecoder : () -> Decode.Decoder Value
buildDecoder _ =
    Decode.oneOf
        [ numDecoder
        , blnDecoder
        , txtDecoder
        , nilDecoder
        , buildLstDecoder ()
        , buildDctDecoder ()
        ]


{-| decoder is our generic value decoder that can be used Json.Decode
-}
decoder : Decode.Decoder Value
decoder =
    buildDecoder ()



-- num decoding


numDecoder : Decode.Decoder Value
numDecoder =
    Decode.map Num Decode.float



-- text decoding


txtDecoder : Decode.Decoder Value
txtDecoder =
    Decode.map Txt Decode.string



-- boolean decoding


blnDecoder : Decode.Decoder Value
blnDecoder =
    Decode.map Bln Decode.bool



-- nil decoding


nilDecoder : Decode.Decoder Value
nilDecoder =
    Decode.null Nil



-- list decoding


{-| builds the lst decoder

>> is function composition
so the `andThen` will pass it the value ()

(buildDecoder >> Decode.list >> Decode.map Lst)

so what is happening is this

   1. buildDecoder () -> Decoder Value
   2. Decode.list #1 -> Decoder (List Value)
   3. Decode.map Lst #2 -> Decoder Value

-}
buildLstDecoder : () -> Decode.Decoder Value
buildLstDecoder _ =
    Decode.succeed () `Decode.andThen` (buildDecoder >> Decode.list >> Decode.map Lst)


lstDecoder : Decode.Decoder Value
lstDecoder =
    buildLstDecoder ()



-- dict decoding


buildDctDecoder : () -> Decode.Decoder Value
buildDctDecoder _ =
    Decode.succeed () `Decode.andThen` (buildDecoder >> dictDecoder >> Decode.map Dct)


dictDecoder : Decode.Decoder a -> Decode.Decoder (Dict.OrderedDict String a)
dictDecoder decoder =
    Decode.map Dict.fromList (Decode.keyValuePairs decoder)
